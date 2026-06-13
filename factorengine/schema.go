package factorengine

import "fmt"

func ResolveFactorRef(ref FactorRef, factor *FactorDefinition) (*FieldSchema, error) {
	if factor == nil {
		return nil, ValidationError{Code: ErrFactorRefNotFound, Field: ref.FactorCode, Message: "factor definition not found"}
	}

	path := ref.PathSegments
	if len(path) == 0 {
		if schemaHasSingleValueField(factor.OutputSchema) {
			path = []string{"value"}
		} else {
			return nil, ValidationError{Code: ErrFactorRefAmbiguous, Field: ref.FactorCode, Message: "multi-field factor requires explicit field reference"}
		}
	}

	field, err := resolveSchemaPath(factor.OutputSchema, path)
	if err != nil {
		return nil, err
	}
	return field, nil
}

func ResolveResponsePath(path ResponsePath, response map[string]*TypeDescriptor) (*TypeDescriptor, error) {
	current := response
	var node *TypeDescriptor
	for i, segment := range path.Segments {
		next, ok := current[segment]
		if !ok {
			return nil, ValidationError{Code: ErrResponsePathInvalid, Field: path.Raw, Message: fmt.Sprintf("segment %q not found", segment)}
		}
		node = next
		if i == len(path.Segments)-1 {
			return node, nil
		}
		if node.Type != ValueTypeObject {
			return nil, ValidationError{Code: ErrResponsePathInvalid, Field: path.Raw, Message: fmt.Sprintf("segment %q is not an object", segment)}
		}
		current = node.Fields
	}
	return nil, ValidationError{Code: ErrResponsePathInvalid, Field: path.Raw, Message: "empty path"}
}

func resolveSchemaPath(schema Schema, path []string) (*FieldSchema, error) {
	var node *FieldSchema
	current := schema
	for i, segment := range path {
		next, ok := current[segment]
		if !ok {
			return nil, ValidationError{Code: ErrFieldNotFound, Field: segment, Message: fmt.Sprintf("schema field %q not found", segment)}
		}
		node = next
		if i == len(path)-1 {
			return node, nil
		}
		if node.Type != ValueTypeObject {
			return nil, ValidationError{Code: ErrFieldNotFound, Field: segment, Message: "intermediate field is not an object"}
		}
		current = node.Fields
	}
	return nil, ValidationError{Code: ErrFieldNotFound, Message: "empty schema path"}
}

func schemaHasSingleValueField(schema Schema) bool {
	if len(schema) != 1 {
		return false
	}
	_, ok := schema["value"]
	return ok
}
