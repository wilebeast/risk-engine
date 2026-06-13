package factorengine

import "fmt"

type compiledExprSnapshot struct {
	Source     string                  `json:"source"`
	ResultType ValueType               `json:"result_type"`
	Bytecode   bytecodeProgramSnapshot `json:"bytecode"`
}

type bytecodeProgramSnapshot struct {
	Instructions []Instruction      `json:"instructions"`
	Constants    []valueSnapshot    `json:"constants"`
	Accessors    []accessorSnapshot `json:"accessors"`
	Builtins     []string           `json:"builtins"`
	ResultType   ValueType          `json:"result_type"`
}

type accessorSnapshot struct {
	Kind       string         `json:"kind"`
	FactorCode string         `json:"factor_code"`
	Steps      []AccessorStep `json:"steps,omitempty"`
}

type valueSnapshot struct {
	Kind   string                   `json:"kind"`
	Bool   bool                     `json:"bool,omitempty"`
	String string                   `json:"string,omitempty"`
	Int    int                      `json:"int,omitempty"`
	Int64  int64                    `json:"int64,omitempty"`
	Double float64                  `json:"double,omitempty"`
	List   []valueSnapshot          `json:"list,omitempty"`
	Map    map[string]valueSnapshot `json:"map,omitempty"`
}

func snapshotCompiledExpr(expr CompiledExpr) (compiledExprSnapshot, error) {
	bytecode, err := snapshotBytecodeProgram(expr.Bytecode())
	if err != nil {
		return compiledExprSnapshot{}, err
	}
	return compiledExprSnapshot{
		Source:     expr.Source(),
		ResultType: expr.ResultType(),
		Bytecode:   bytecode,
	}, nil
}

func restoreCompiledExpr(snapshot compiledExprSnapshot) (CompiledExpr, error) {
	bytecode, err := restoreBytecodeProgram(snapshot.Bytecode)
	if err != nil {
		return nil, err
	}
	return compiledExpr{
		source:     snapshot.Source,
		bytecode:   bytecode,
		resultType: snapshot.ResultType,
	}, nil
}

func snapshotBytecodeProgram(program BytecodeProgram) (bytecodeProgramSnapshot, error) {
	constants := make([]valueSnapshot, 0, len(program.Constants))
	for _, constant := range program.Constants {
		snapshot, err := snapshotValue(constant)
		if err != nil {
			return bytecodeProgramSnapshot{}, err
		}
		constants = append(constants, snapshot)
	}

	accessors := make([]accessorSnapshot, 0, len(program.Accessors))
	for _, accessor := range program.Accessors {
		snapshot, err := snapshotAccessor(accessor)
		if err != nil {
			return bytecodeProgramSnapshot{}, err
		}
		accessors = append(accessors, snapshot)
	}

	return bytecodeProgramSnapshot{
		Instructions: append([]Instruction(nil), program.Instructions...),
		Constants:    constants,
		Accessors:    accessors,
		Builtins:     append([]string(nil), program.Builtins...),
		ResultType:   program.ResultType,
	}, nil
}

func restoreBytecodeProgram(snapshot bytecodeProgramSnapshot) (BytecodeProgram, error) {
	constants := make([]any, 0, len(snapshot.Constants))
	for _, constant := range snapshot.Constants {
		value, err := restoreValue(constant)
		if err != nil {
			return BytecodeProgram{}, err
		}
		constants = append(constants, value)
	}

	accessors := make([]ValueAccessor, 0, len(snapshot.Accessors))
	for _, accessor := range snapshot.Accessors {
		value, err := restoreAccessor(accessor)
		if err != nil {
			return BytecodeProgram{}, err
		}
		accessors = append(accessors, value)
	}

	return BytecodeProgram{
		Instructions: append([]Instruction(nil), snapshot.Instructions...),
		Constants:    constants,
		Accessors:    accessors,
		Builtins:     append([]string(nil), snapshot.Builtins...),
		ResultType:   snapshot.ResultType,
	}, nil
}

func snapshotAccessor(accessor ValueAccessor) (accessorSnapshot, error) {
	switch value := accessor.(type) {
	case FactorValueAccessor:
		return accessorSnapshot{
			Kind:       "factor",
			FactorCode: value.FactorCode,
			Steps:      append([]AccessorStep(nil), value.Steps...),
		}, nil
	default:
		return accessorSnapshot{}, ValidationError{
			Code:    ErrDefinitionInvalid,
			Field:   "accessor",
			Message: fmt.Sprintf("unsupported accessor type %T", accessor),
		}
	}
}

func restoreAccessor(snapshot accessorSnapshot) (ValueAccessor, error) {
	switch snapshot.Kind {
	case "factor":
		return FactorValueAccessor{
			FactorCode: snapshot.FactorCode,
			Steps:      append([]AccessorStep(nil), snapshot.Steps...),
		}, nil
	default:
		return nil, ValidationError{
			Code:    ErrDefinitionInvalid,
			Field:   "accessor",
			Message: fmt.Sprintf("unsupported accessor snapshot kind %q", snapshot.Kind),
		}
	}
}

func snapshotValue(value any) (valueSnapshot, error) {
	switch v := value.(type) {
	case nil:
		return valueSnapshot{Kind: "null"}, nil
	case bool:
		return valueSnapshot{Kind: "bool", Bool: v}, nil
	case string:
		return valueSnapshot{Kind: "string", String: v}, nil
	case int:
		return valueSnapshot{Kind: "int", Int: v}, nil
	case int64:
		return valueSnapshot{Kind: "int64", Int64: v}, nil
	case float64:
		return valueSnapshot{Kind: "double", Double: v}, nil
	case []any:
		list := make([]valueSnapshot, 0, len(v))
		for _, item := range v {
			snapshot, err := snapshotValue(item)
			if err != nil {
				return valueSnapshot{}, err
			}
			list = append(list, snapshot)
		}
		return valueSnapshot{Kind: "list", List: list}, nil
	case map[string]any:
		mapped := make(map[string]valueSnapshot, len(v))
		for key, item := range v {
			snapshot, err := snapshotValue(item)
			if err != nil {
				return valueSnapshot{}, err
			}
			mapped[key] = snapshot
		}
		return valueSnapshot{Kind: "map", Map: mapped}, nil
	default:
		return valueSnapshot{}, ValidationError{
			Code:    ErrDefinitionInvalid,
			Field:   "constant",
			Message: fmt.Sprintf("unsupported constant type %T", value),
		}
	}
}

func restoreValue(snapshot valueSnapshot) (any, error) {
	switch snapshot.Kind {
	case "null":
		return nil, nil
	case "bool":
		return snapshot.Bool, nil
	case "string":
		return snapshot.String, nil
	case "int":
		return snapshot.Int, nil
	case "int64":
		return snapshot.Int64, nil
	case "double":
		return snapshot.Double, nil
	case "list":
		list := make([]any, 0, len(snapshot.List))
		for _, item := range snapshot.List {
			value, err := restoreValue(item)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		return list, nil
	case "map":
		mapped := make(map[string]any, len(snapshot.Map))
		for key, item := range snapshot.Map {
			value, err := restoreValue(item)
			if err != nil {
				return nil, err
			}
			mapped[key] = value
		}
		return mapped, nil
	default:
		return nil, ValidationError{
			Code:    ErrDefinitionInvalid,
			Field:   "constant",
			Message: fmt.Sprintf("unsupported value snapshot kind %q", snapshot.Kind),
		}
	}
}
