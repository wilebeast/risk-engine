package factorengine

type ValueAccessor interface {
	Get(ctx EvalContext) (any, bool)
}

type FactorValueAccessor struct {
	FactorCode string
	Steps      []AccessorStep
}

type AccessorStep struct {
	Key string
}

func CompileFactorAccessor(ref FactorRef, factor *FactorDefinition) (ValueAccessor, error) {
	if _, err := ResolveFactorRef(ref, factor); err != nil {
		return nil, err
	}

	path := ref.PathSegments
	if len(path) == 0 && schemaHasSingleValueField(factor.OutputSchema) {
		path = []string{"value"}
	}

	steps := make([]AccessorStep, 0, len(path))
	for _, segment := range path {
		steps = append(steps, AccessorStep{Key: segment})
	}

	return FactorValueAccessor{
		FactorCode: ref.FactorCode,
		Steps:      steps,
	}, nil
}

func (a FactorValueAccessor) Get(ctx EvalContext) (any, bool) {
	current, ok := ctx[a.FactorCode]
	if !ok {
		return nil, false
	}
	for idx, step := range a.Steps {
		if idx == 0 && step.Key == "value" {
			if obj, ok := current.(map[string]any); ok {
				next, ok := obj[step.Key]
				if !ok {
					return nil, false
				}
				current = next
				continue
			}
			return current, true
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := obj[step.Key]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}
