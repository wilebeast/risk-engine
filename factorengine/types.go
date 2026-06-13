package factorengine

type FactorCategory string

const (
	FactorCategoryRPC FactorCategory = "RPC"
)

type FactorType string

const (
	FactorTypeThriftInvoker FactorType = "THRIFT_INVOKER"
)

type ValueType string

const (
	ValueTypeString ValueType = "STRING"
	ValueTypeInt    ValueType = "INT"
	ValueTypeLong   ValueType = "LONG"
	ValueTypeBool   ValueType = "BOOL"
	ValueTypeDouble ValueType = "DOUBLE"
	ValueTypeObject ValueType = "OBJECT"
	ValueTypeList   ValueType = "LIST"
	ValueTypeMap    ValueType = "MAP"
	ValueTypeNull   ValueType = "NULL"
)

type FieldSchema struct {
	Type     ValueType               `json:"type"`
	Required bool                    `json:"required"`
	Fields   map[string]*FieldSchema `json:"fields,omitempty"`
	ElemType *FieldSchema            `json:"elem_type,omitempty"`
}

type Schema map[string]*FieldSchema

type FactorDefinition struct {
	FactorCode     string          `json:"factor_code"`
	FactorCategory FactorCategory  `json:"factor_category"`
	FactorType     FactorType      `json:"factor_type"`
	InputSchema    Schema          `json:"input_schema"`
	OutputSchema   Schema          `json:"output_schema"`
	Dependencies   []string        `json:"dependencies"`
	Config         RPCFactorConfig `json:"config"`
	Published      bool            `json:"published"`
}

type RPCFactorConfig struct {
	IDLCode         string            `json:"idl_code"`
	IDLVersion      int               `json:"idl_version"`
	Service         string            `json:"service"`
	Method          string            `json:"method"`
	RequestMapping  map[string]string `json:"request_mapping"`
	ResponseMapping map[string]string `json:"response_mapping"`
}

type FactorRef struct {
	FactorCode        string
	PathSegments      []string
	IsDefaultValueRef bool
}

type ResponsePath struct {
	Raw      string
	Segments []string
}

type ValidationErrorCode string

const (
	ErrFactorRefNotFound    ValidationErrorCode = "FACTOR_REF_NOT_FOUND"
	ErrFactorRefAmbiguous   ValidationErrorCode = "FACTOR_REF_AMBIGUOUS"
	ErrFactorRefInvalid     ValidationErrorCode = "FACTOR_REF_INVALID"
	ErrFieldNotFound        ValidationErrorCode = "FIELD_NOT_FOUND"
	ErrRequestFieldMissing  ValidationErrorCode = "REQUEST_FIELD_MISSING"
	ErrResponsePathInvalid  ValidationErrorCode = "RESPONSE_PATH_INVALID"
	ErrTypeMismatch         ValidationErrorCode = "TYPE_MISMATCH"
	ErrDefinitionInvalid    ValidationErrorCode = "DEFINITION_INVALID"
	ErrDependencyMismatch   ValidationErrorCode = "DEPENDENCY_MISMATCH"
	ErrFactorNotPublished   ValidationErrorCode = "FACTOR_NOT_PUBLISHED"
	ErrUnsupportedValueType ValidationErrorCode = "UNSUPPORTED_VALUE_TYPE"
	ErrExpressionInvalid    ValidationErrorCode = "EXPRESSION_INVALID"
	ErrUnknownIdentifier    ValidationErrorCode = "UNKNOWN_IDENTIFIER"
	ErrUnsupportedOperator  ValidationErrorCode = "UNSUPPORTED_OPERATOR"
	ErrUnknownFunction      ValidationErrorCode = "UNKNOWN_FUNCTION"
	ErrInvalidArgumentCount ValidationErrorCode = "INVALID_ARGUMENT_COUNT"
	ErrDivisionByZero       ValidationErrorCode = "DIVISION_BY_ZERO"
	ErrExecutionBudget      ValidationErrorCode = "EXECUTION_BUDGET_EXCEEDED"
	ErrExecutionCancelled   ValidationErrorCode = "EXECUTION_CANCELLED"
	ErrInternalPanic        ValidationErrorCode = "INTERNAL_PANIC"
)

type ValidationError struct {
	Code    ValidationErrorCode
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return string(e.Code) + ": " + e.Message
	}
	return string(e.Code) + " [" + e.Field + "]: " + e.Message
}

type TypeDescriptor struct {
	Type     ValueType
	Required bool
	Fields   map[string]*TypeDescriptor
}

type ThriftMethod struct {
	Request  map[string]*TypeDescriptor
	Response map[string]*TypeDescriptor
}

type ThriftIDL interface {
	LookupMethod(idlCode string, version int, service, method string) (*ThriftMethod, bool)
}
