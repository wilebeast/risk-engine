package factorengine

import (
	"fmt"
)

type FactorRegistry map[string]*FactorDefinition

func ValidateRPCFactor(def *FactorDefinition, registry FactorRegistry, idl ThriftIDL) []error {
	var errs []error
	if def == nil {
		return []error{ValidationError{Code: ErrDefinitionInvalid, Message: "factor definition is nil"}}
	}

	if def.FactorCategory != FactorCategoryRPC {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "factor_category", Message: "must be RPC"})
	}
	if def.FactorType != FactorTypeThriftInvoker {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "factor_type", Message: "must be THRIFT_INVOKER"})
	}
	if len(def.InputSchema) == 0 {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "input_schema", Message: "input_schema is required"})
	}
	if len(def.OutputSchema) == 0 {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "output_schema", Message: "output_schema is required"})
	}
	if len(def.Config.RequestMapping) == 0 {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "config.request_mapping", Message: "request_mapping is required"})
	}
	if len(def.Config.ResponseMapping) == 0 {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "config.response_mapping", Message: "response_mapping is required"})
	}
	if def.Config.IDLCode == "" || def.Config.IDLVersion == 0 || def.Config.Service == "" || def.Config.Method == "" {
		errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "config", Message: "idl_code, idl_version, service, method are required"})
	}
	if len(errs) > 0 {
		return errs
	}

	method, ok := idl.LookupMethod(def.Config.IDLCode, def.Config.IDLVersion, def.Config.Service, def.Config.Method)
	if !ok {
		return []error{ValidationError{Code: ErrDefinitionInvalid, Field: "config", Message: "thrift method not found"}}
	}

	depSet := make(map[string]struct{}, len(def.Dependencies))
	for _, dep := range def.Dependencies {
		depSet[dep] = struct{}{}
	}

	for requestField, factorRefRaw := range def.Config.RequestMapping {
		ref, err := ParseFactorRef(factorRefRaw)
		if err != nil {
			errs = append(errs, wrapFieldError(err, "config.request_mapping."+requestField))
			continue
		}

		depFactor, ok := registry[ref.FactorCode]
		if !ok {
			errs = append(errs, ValidationError{Code: ErrFactorRefNotFound, Field: requestField, Message: fmt.Sprintf("dependency factor %q not found", ref.FactorCode)})
			continue
		}
		if _, ok := depSet[ref.FactorCode]; !ok {
			errs = append(errs, ValidationError{Code: ErrDependencyMismatch, Field: requestField, Message: fmt.Sprintf("factor %q missing from dependencies", ref.FactorCode)})
		}
		if !depFactor.Published {
			errs = append(errs, ValidationError{Code: ErrFactorNotPublished, Field: requestField, Message: fmt.Sprintf("factor %q is not published", ref.FactorCode)})
		}

		sourceField, err := ResolveFactorRef(ref, depFactor)
		if err != nil {
			errs = append(errs, wrapFieldError(err, "config.request_mapping."+requestField))
			continue
		}

		targetField, ok := method.Request[requestField]
		if !ok {
			errs = append(errs, ValidationError{Code: ErrRequestFieldMissing, Field: requestField, Message: "request field not found in thrift method"})
			continue
		}

		if err := ensureTypeCompatible(sourceField.Type, targetField.Type, "config.request_mapping."+requestField); err != nil {
			errs = append(errs, err)
		}
	}

	covered := make(map[string]struct{}, len(def.Config.ResponseMapping))
	for outputField, responsePathRaw := range def.Config.ResponseMapping {
		targetSchema, ok := def.OutputSchema[outputField]
		if !ok {
			errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "config.response_mapping." + outputField, Message: "output field missing from output_schema"})
			continue
		}
		covered[outputField] = struct{}{}

		path, err := ParseResponsePath(responsePathRaw)
		if err != nil {
			errs = append(errs, wrapFieldError(err, "config.response_mapping."+outputField))
			continue
		}
		sourceType, err := ResolveResponsePath(path, method.Response)
		if err != nil {
			errs = append(errs, wrapFieldError(err, "config.response_mapping."+outputField))
			continue
		}
		if err := ensureTypeCompatible(targetSchema.Type, sourceType.Type, "config.response_mapping."+outputField); err != nil {
			errs = append(errs, err)
		}
		if targetSchema.Required && !sourceType.Required {
			errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "config.response_mapping." + outputField, Message: "required output field cannot map from optional response field"})
		}
	}

	for outputField := range def.OutputSchema {
		if _, ok := covered[outputField]; !ok {
			errs = append(errs, ValidationError{Code: ErrDefinitionInvalid, Field: "output_schema." + outputField, Message: "output field missing from response_mapping"})
		}
	}

	return errs
}

func ensureTypeCompatible(source, target ValueType, field string) error {
	if source == target {
		return nil
	}
	switch source {
	case ValueTypeInt:
		if target == ValueTypeLong {
			return nil
		}
	case ValueTypeObject:
		return ValidationError{Code: ErrUnsupportedValueType, Field: field, Message: "object types are not allowed as terminal values"}
	}
	return ValidationError{Code: ErrTypeMismatch, Field: field, Message: fmt.Sprintf("type mismatch: %s -> %s", source, target)}
}

func wrapFieldError(err error, field string) error {
	if v, ok := err.(ValidationError); ok {
		v.Field = field
		return v
	}
	return err
}
