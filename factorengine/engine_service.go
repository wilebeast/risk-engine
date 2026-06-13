package factorengine

import (
	"fmt"
	"sort"
	"sync"
)

type RuleDefinition struct {
	RuleCode   string `json:"rule_code"`
	Expression string `json:"expression"`
}

type RuleSetDefinition struct {
	RuleSetCode string           `json:"rule_set_code"`
	Version     int              `json:"version"`
	Rules       []RuleDefinition `json:"rules"`
}

type PublishedRuleSet struct {
	Definition RuleSetDefinition
	Engine     *Engine
	Compiled   map[string]CompiledExpr
}

type EngineServiceConfig struct {
	Registry FactorRegistry
	Compiler RuleCompiler
	Cache    ProgramCache
	Observer RuleObserver
	VMConfig BytecodeVMConfig
}

type EngineService struct {
	registry FactorRegistry
	compiler RuleCompiler
	cache    ProgramCache
	observer RuleObserver
	vmConfig BytecodeVMConfig

	mu       sync.RWMutex
	ruleSets map[string]map[int]*PublishedRuleSet
	latest   map[string]int
}

func NewEngineService(config EngineServiceConfig) *EngineService {
	observer := config.Observer
	if observer == nil {
		observer = NoopRuleObserver{}
	}
	compiler := config.Compiler
	if compiler == nil {
		compiler = NewRuleCompiler()
	}
	return &EngineService{
		registry: config.Registry,
		compiler: compiler,
		cache:    config.Cache,
		observer: observer,
		vmConfig: config.VMConfig,
		ruleSets: make(map[string]map[int]*PublishedRuleSet),
		latest:   make(map[string]int),
	}
}

func (s *EngineService) PublishRuleSet(def RuleSetDefinition) (*PublishedRuleSet, error) {
	if def.RuleSetCode == "" {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: "rule_set_code", Message: "rule_set_code is required"}
	}
	if def.Version <= 0 {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: "version", Message: "version must be positive"}
	}
	if len(def.Rules) == 0 {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: "rules", Message: "at least one rule is required"}
	}

	engine := NewEngine(EngineConfig{
		Registry: s.registry,
		Compiler: s.compiler,
		Cache:    s.cache,
		Observer: s.observer,
		VMConfig: s.vmConfig,
	})
	compiled := make(map[string]CompiledExpr, len(def.Rules))
	seen := make(map[string]struct{}, len(def.Rules))
	for _, rule := range def.Rules {
		if rule.RuleCode == "" {
			return nil, ValidationError{Code: ErrDefinitionInvalid, Field: "rule_code", Message: "rule_code is required"}
		}
		if rule.Expression == "" {
			return nil, ValidationError{Code: ErrDefinitionInvalid, Field: rule.RuleCode, Message: "expression is required"}
		}
		if _, ok := seen[rule.RuleCode]; ok {
			return nil, ValidationError{Code: ErrDefinitionInvalid, Field: rule.RuleCode, Message: "duplicate rule_code"}
		}
		seen[rule.RuleCode] = struct{}{}

		expr, err := engine.Compile(rule.Expression)
		if err != nil {
			return nil, wrapFieldError(err, "rules."+rule.RuleCode)
		}
		compiled[rule.RuleCode] = expr
	}

	published := &PublishedRuleSet{
		Definition: def,
		Engine:     engine,
		Compiled:   compiled,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	versions, ok := s.ruleSets[def.RuleSetCode]
	if !ok {
		versions = make(map[int]*PublishedRuleSet)
		s.ruleSets[def.RuleSetCode] = versions
	}
	versions[def.Version] = published
	if latest, ok := s.latest[def.RuleSetCode]; !ok || def.Version >= latest {
		s.latest[def.RuleSetCode] = def.Version
	}
	return published, nil
}

func (s *EngineService) LoadRuleSet(ruleSetCode string, version int) (*PublishedRuleSet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.ruleSets[ruleSetCode]
	if !ok {
		return nil, false
	}
	ruleSet, ok := versions[version]
	return ruleSet, ok
}

func (s *EngineService) LoadLatestRuleSet(ruleSetCode string) (*PublishedRuleSet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	version, ok := s.latest[ruleSetCode]
	if !ok {
		return nil, false
	}
	ruleSet, ok := s.ruleSets[ruleSetCode][version]
	return ruleSet, ok
}

func (s *EngineService) EvalRule(ruleSetCode string, version int, ruleCode string, ctx EvalContext) (any, error) {
	ruleSet, ok := s.LoadRuleSet(ruleSetCode, version)
	if !ok {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: ruleSetCode, Message: fmt.Sprintf("rule set version %d not found", version)}
	}
	compiled, ok := ruleSet.Compiled[ruleCode]
	if !ok {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: ruleCode, Message: "rule not found"}
	}
	return compiled.Eval(ctx)
}

func (s *EngineService) EvalLatestRule(ruleSetCode string, ruleCode string, ctx EvalContext) (any, error) {
	ruleSet, ok := s.LoadLatestRuleSet(ruleSetCode)
	if !ok {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: ruleSetCode, Message: "latest rule set not found"}
	}
	compiled, ok := ruleSet.Compiled[ruleCode]
	if !ok {
		return nil, ValidationError{Code: ErrDefinitionInvalid, Field: ruleCode, Message: "rule not found"}
	}
	return compiled.Eval(ctx)
}

func (s *EngineService) ListRuleSetVersions(ruleSetCode string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions := s.ruleSets[ruleSetCode]
	result := make([]int, 0, len(versions))
	for version := range versions {
		result = append(result, version)
	}
	sort.Ints(result)
	return result
}
