package factorengine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type ProgramCache interface {
	Get(key string) (CompiledExpr, bool)
	Set(key string, expr CompiledExpr)
}

type InMemoryProgramCache struct {
	mu    sync.RWMutex
	items map[string]CompiledExpr
}

func NewInMemoryProgramCache() *InMemoryProgramCache {
	return &InMemoryProgramCache{
		items: make(map[string]CompiledExpr),
	}
}

func (c *InMemoryProgramCache) Get(key string) (CompiledExpr, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	expr, ok := c.items[key]
	return expr, ok
}

func (c *InMemoryProgramCache) Set(key string, expr CompiledExpr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = expr
}

type CachedRuleCompiler struct {
	inner RuleCompiler
	cache ProgramCache
}

func NewCachedRuleCompiler(inner RuleCompiler, cache ProgramCache) RuleCompiler {
	if inner == nil {
		inner = NewRuleCompiler()
	}
	if cache == nil {
		cache = NewInMemoryProgramCache()
	}
	return CachedRuleCompiler{
		inner: inner,
		cache: cache,
	}
}

func (c CachedRuleCompiler) TypeCheck(expr string, registry FactorRegistry) (ValueType, error) {
	return c.inner.TypeCheck(expr, registry)
}

func (c CachedRuleCompiler) Compile(expr string, registry FactorRegistry) (CompiledExpr, error) {
	key, err := BuildCompileCacheKey(expr, registry)
	if err != nil {
		return nil, err
	}
	if compiled, ok := c.cache.Get(key); ok {
		return compiled, nil
	}
	compiled, err := c.inner.Compile(expr, registry)
	if err != nil {
		return nil, err
	}
	c.cache.Set(key, compiled)
	return compiled, nil
}

func BuildCompileCacheKey(expr string, registry FactorRegistry) (string, error) {
	registryFingerprint, err := RegistryFingerprint(registry)
	if err != nil {
		return "", err
	}
	return expr + "::" + registryFingerprint, nil
}

func RegistryFingerprint(registry FactorRegistry) (string, error) {
	keys := make([]string, 0, len(registry))
	for factorCode := range registry {
		keys = append(keys, factorCode)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, factorCode := range keys {
		def := registry[factorCode]
		builder.WriteString(factorCode)
		builder.WriteByte('|')
		builder.WriteString(string(def.FactorCategory))
		builder.WriteByte('|')
		builder.WriteString(string(def.FactorType))
		builder.WriteByte('|')
		builder.WriteString(strconv.FormatBool(def.Published))
		builder.WriteByte('|')

		inputSchemaFingerprint, err := schemaFingerprint(def.InputSchema)
		if err != nil {
			return "", err
		}
		builder.WriteString(inputSchemaFingerprint)
		builder.WriteByte('|')

		outputSchemaFingerprint, err := schemaFingerprint(def.OutputSchema)
		if err != nil {
			return "", err
		}
		builder.WriteString(outputSchemaFingerprint)
		builder.WriteByte('|')

		deps := append([]string(nil), def.Dependencies...)
		sort.Strings(deps)
		builder.WriteString(strings.Join(deps, ","))
		builder.WriteByte('|')

		configFingerprint, err := rpcConfigFingerprint(def.Config)
		if err != nil {
			return "", err
		}
		builder.WriteString(configFingerprint)
		builder.WriteByte(';')
	}
	return builder.String(), nil
}

func schemaFingerprint(schema Schema) (string, error) {
	if len(schema) == 0 {
		return "{}", nil
	}

	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		fieldFingerprint, err := fieldSchemaFingerprint(schema[key])
		if err != nil {
			return "", err
		}
		parts = append(parts, key+":"+fieldFingerprint)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func fieldSchemaFingerprint(field *FieldSchema) (string, error) {
	if field == nil {
		return "null", nil
	}

	var builder strings.Builder
	builder.WriteString(string(field.Type))
	builder.WriteByte(':')
	builder.WriteString(strconv.FormatBool(field.Required))

	if field.ElemType != nil {
		elemFingerprint, err := fieldSchemaFingerprint(field.ElemType)
		if err != nil {
			return "", err
		}
		builder.WriteString(":elem=")
		builder.WriteString(elemFingerprint)
	}
	if len(field.Fields) > 0 {
		nestedSchema := make(Schema, len(field.Fields))
		for key, value := range field.Fields {
			nestedSchema[key] = value
		}
		nestedFingerprint, err := schemaFingerprint(nestedSchema)
		if err != nil {
			return "", err
		}
		builder.WriteString(":fields=")
		builder.WriteString(nestedFingerprint)
	}
	return builder.String(), nil
}

func rpcConfigFingerprint(config RPCFactorConfig) (string, error) {
	requestMapping, err := stableJSON(config.RequestMapping)
	if err != nil {
		return "", err
	}
	responseMapping, err := stableJSON(config.ResponseMapping)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s|%d|%s|%s|%s|%s",
		config.IDLCode,
		config.IDLVersion,
		config.Service,
		config.Method,
		requestMapping,
		responseMapping,
	), nil
}

func stableJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
