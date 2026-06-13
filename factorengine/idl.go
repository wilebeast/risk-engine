package factorengine

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type MemoryIDLRegistry struct {
	methods map[string]*ThriftMethod
}

func NewMemoryIDLRegistry() *MemoryIDLRegistry {
	return &MemoryIDLRegistry{methods: make(map[string]*ThriftMethod)}
}

func (r *MemoryIDLRegistry) Register(idlCode string, version int, service, method string, spec *ThriftMethod) {
	r.methods[registryKey(idlCode, version, service, method)] = spec
}

func (r *MemoryIDLRegistry) LookupMethod(idlCode string, version int, service, method string) (*ThriftMethod, bool) {
	spec, ok := r.methods[registryKey(idlCode, version, service, method)]
	return spec, ok
}

func registryKey(idlCode string, version int, service, method string) string {
	return fmt.Sprintf("%s#%d#%s#%s", idlCode, version, service, method)
}

func (r *MemoryIDLRegistry) RegisterThriftFile(idlCode string, version int, path string) error {
	specs, err := ParseThriftFile(path)
	if err != nil {
		return err
	}
	for serviceName, methods := range specs {
		for methodName, spec := range methods {
			r.Register(idlCode, version, serviceName, methodName, spec)
		}
	}
	return nil
}

func ParseThriftFile(path string) (map[string]map[string]*ThriftMethod, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseThrift(string(content))
}

func parseThrift(content string) (map[string]map[string]*ThriftMethod, error) {
	cleaned := stripLineComments(content)
	structs, err := parseThriftStructs(cleaned)
	if err != nil {
		return nil, err
	}
	return parseThriftServices(cleaned, structs)
}

func stripLineComments(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func parseThriftStructs(content string) (map[string]map[string]*TypeDescriptor, error) {
	re := regexp.MustCompile(`(?s)struct\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	matches := re.FindAllStringSubmatch(content, -1)
	rawStructs := make(map[string]string, len(matches))
	for _, match := range matches {
		rawStructs[match[1]] = match[2]
	}

	structs := make(map[string]map[string]*TypeDescriptor, len(matches))
	visiting := make(map[string]bool, len(matches))
	for name := range rawStructs {
		if _, err := buildStructDescriptor(name, rawStructs, structs, visiting); err != nil {
			return nil, err
		}
	}
	return structs, nil
}

func parseThriftServices(content string, structs map[string]map[string]*TypeDescriptor) (map[string]map[string]*ThriftMethod, error) {
	re := regexp.MustCompile(`(?s)service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	serviceMatches := re.FindAllStringSubmatch(content, -1)
	services := make(map[string]map[string]*ThriftMethod, len(serviceMatches))
	methodRe := regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_<>, ]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*?)\)`)
	for _, serviceMatch := range serviceMatches {
		serviceName := serviceMatch[1]
		methods := make(map[string]*ThriftMethod)
		for _, line := range splitStatements(serviceMatch[2]) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			methodMatch := methodRe.FindStringSubmatch(line)
			if methodMatch == nil {
				return nil, fmt.Errorf("unsupported thrift method syntax: %q", line)
			}
			returnType := strings.TrimSpace(methodMatch[1])
			methodName := methodMatch[2]
			requestFields, err := parseThriftFields(methodMatch[3], structs)
			if err != nil {
				return nil, err
			}
			responseFields, err := resolveThriftType(returnType, structs)
			if err != nil {
				return nil, err
			}
			methods[methodName] = &ThriftMethod{
				Request:  requestFields,
				Response: responseFields,
			}
		}
		services[serviceName] = methods
	}
	return services, nil
}

func parseThriftFields(content string, structs map[string]map[string]*TypeDescriptor) (map[string]*TypeDescriptor, error) {
	fields := make(map[string]*TypeDescriptor)
	for _, rawField := range splitStatements(content) {
		rawField = strings.TrimSpace(rawField)
		if rawField == "" {
			continue
		}
		field, err := parseThriftField(rawField, structs)
		if err != nil {
			return nil, err
		}
		fields[field.name] = field.desc
	}
	return fields, nil
}

type thriftField struct {
	name string
	desc *TypeDescriptor
}

func parseThriftField(raw string, structs map[string]map[string]*TypeDescriptor) (*thriftField, error) {
	re := regexp.MustCompile(`^\s*\d+\s*:\s*(required|optional)?\s*([A-Za-z_][A-Za-z0-9_<>,]*)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	match := re.FindStringSubmatch(raw)
	if match == nil {
		return nil, fmt.Errorf("unsupported thrift field syntax: %q", raw)
	}
	required := match[1] == "required"
	typeName := match[2]
	fieldName := match[3]
	desc, err := resolveThriftTypeDescriptor(typeName, required, structs)
	if err != nil {
		return nil, err
	}
	return &thriftField{name: fieldName, desc: desc}, nil
}

func resolveThriftType(typeName string, structs map[string]map[string]*TypeDescriptor) (map[string]*TypeDescriptor, error) {
	desc, err := resolveThriftTypeDescriptor(typeName, true, structs)
	if err != nil {
		return nil, err
	}
	if desc.Type != ValueTypeObject {
		return nil, fmt.Errorf("thrift return type %q must be a struct", typeName)
	}
	return desc.Fields, nil
}

func resolveThriftTypeDescriptor(typeName string, required bool, structs map[string]map[string]*TypeDescriptor) (*TypeDescriptor, error) {
	switch strings.TrimSpace(typeName) {
	case "string":
		return &TypeDescriptor{Type: ValueTypeString, Required: required}, nil
	case "i32":
		return &TypeDescriptor{Type: ValueTypeInt, Required: required}, nil
	case "i64":
		return &TypeDescriptor{Type: ValueTypeLong, Required: required}, nil
	case "bool":
		return &TypeDescriptor{Type: ValueTypeBool, Required: required}, nil
	case "double":
		return &TypeDescriptor{Type: ValueTypeDouble, Required: required}, nil
	}
	fields, ok := structs[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown thrift type %q", typeName)
	}
	return &TypeDescriptor{
		Type:     ValueTypeObject,
		Required: required,
		Fields:   cloneTypeDescriptors(fields),
	}, nil
}

func cloneTypeDescriptors(fields map[string]*TypeDescriptor) map[string]*TypeDescriptor {
	cloned := make(map[string]*TypeDescriptor, len(fields))
	for key, field := range fields {
		next := &TypeDescriptor{
			Type:     field.Type,
			Required: field.Required,
		}
		if len(field.Fields) > 0 {
			next.Fields = cloneTypeDescriptors(field.Fields)
		}
		cloned[key] = next
	}
	return cloned
}

func splitStatements(content string) []string {
	content = strings.ReplaceAll(content, ";", "\n")
	lines := strings.Split(content, "\n")
	var statements []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		statements = append(statements, line)
	}
	return statements
}

func buildStructDescriptor(name string, rawStructs map[string]string, structs map[string]map[string]*TypeDescriptor, visiting map[string]bool) (map[string]*TypeDescriptor, error) {
	if fields, ok := structs[name]; ok {
		return fields, nil
	}
	if visiting[name] {
		return nil, fmt.Errorf("cyclic thrift struct reference detected: %s", name)
	}

	body, ok := rawStructs[name]
	if !ok {
		return nil, fmt.Errorf("unknown thrift struct %q", name)
	}

	visiting[name] = true
	fields, err := parseThriftFields(body, lazyStructs(rawStructs, structs, visiting))
	delete(visiting, name)
	if err != nil {
		return nil, err
	}
	structs[name] = fields
	return fields, nil
}

func lazyStructs(rawStructs map[string]string, structs map[string]map[string]*TypeDescriptor, visiting map[string]bool) map[string]map[string]*TypeDescriptor {
	lazy := make(map[string]map[string]*TypeDescriptor, len(rawStructs))
	for name := range rawStructs {
		if fields, ok := structs[name]; ok {
			lazy[name] = fields
			continue
		}
		fields, err := buildStructDescriptor(name, rawStructs, structs, visiting)
		if err == nil {
			lazy[name] = fields
		}
	}
	return lazy
}
