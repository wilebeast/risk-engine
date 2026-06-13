package factorengine

import "fmt"

type BoundExpr interface {
	boundNode()
	ResultType() ValueType
	Eval(ctx EvalContext) (any, error)
}

type BoundLiteralExpr struct {
	Value any
	Type  ValueType
	Field *FieldSchema
}

func (BoundLiteralExpr) boundNode() {}

func (e BoundLiteralExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundLiteralExpr) Eval(ctx EvalContext) (any, error) {
	return e.Value, nil
}

type BoundFactorRefExpr struct {
	Ref      FactorRef
	Accessor ValueAccessor
	Type     ValueType
	Field    *FieldSchema
}

func (BoundFactorRefExpr) boundNode() {}

func (e BoundFactorRefExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundFactorRefExpr) Eval(ctx EvalContext) (any, error) {
	value, ok := e.Accessor.Get(ctx)
	if !ok {
		return nil, nil
	}
	return value, nil
}

type BoundUnaryExpr struct {
	Op        string
	Expr      BoundExpr
	Type      ValueType
	Field     *FieldSchema
	Evaluator func(any) (any, error)
}

func (BoundUnaryExpr) boundNode() {}

func (e BoundUnaryExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundUnaryExpr) Eval(ctx EvalContext) (any, error) {
	value, err := e.Expr.Eval(ctx)
	if err != nil {
		return nil, err
	}
	return e.Evaluator(value)
}

type BoundBinaryExpr struct {
	Op        string
	Left      BoundExpr
	Right     BoundExpr
	Type      ValueType
	Field     *FieldSchema
	Evaluator func(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error)
}

func (BoundBinaryExpr) boundNode() {}

func (e BoundBinaryExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundBinaryExpr) Eval(ctx EvalContext) (any, error) {
	return e.Evaluator(e.Left, e.Right, ctx)
}

type BoundConditionalExpr struct {
	Cond      BoundExpr
	Then      BoundExpr
	Else      BoundExpr
	Type      ValueType
	Field     *FieldSchema
	Evaluator func(cond BoundExpr, thenExpr BoundExpr, elseExpr BoundExpr, ctx EvalContext) (any, error)
}

func (BoundConditionalExpr) boundNode() {}

func (e BoundConditionalExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundConditionalExpr) Eval(ctx EvalContext) (any, error) {
	return e.Evaluator(e.Cond, e.Then, e.Else, ctx)
}

type BoundFunctionCallExpr struct {
	Name      string
	Args      []BoundExpr
	Type      ValueType
	Field     *FieldSchema
	Evaluator func(args []BoundExpr, ctx EvalContext) (any, error)
}

func (BoundFunctionCallExpr) boundNode() {}

func (e BoundFunctionCallExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundFunctionCallExpr) Eval(ctx EvalContext) (any, error) {
	return e.Evaluator(e.Args, ctx)
}

type BoundListExpr struct {
	Elements []BoundExpr
	Type     ValueType
	ElemType ValueType
	Field    *FieldSchema
}

func (BoundListExpr) boundNode() {}

func (e BoundListExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundListExpr) Eval(ctx EvalContext) (any, error) {
	values := make([]any, 0, len(e.Elements))
	for _, elem := range e.Elements {
		value, err := elem.Eval(ctx)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

type BoundMapEntry struct {
	Key   string
	Value BoundExpr
}

type BoundMapExpr struct {
	Entries   []BoundMapEntry
	Type      ValueType
	ValueType ValueType
	Field     *FieldSchema
}

func (BoundMapExpr) boundNode() {}

func (e BoundMapExpr) ResultType() ValueType {
	return e.Type
}

func (e BoundMapExpr) Eval(ctx EvalContext) (any, error) {
	values := make(map[string]any, len(e.Entries))
	for _, entry := range e.Entries {
		value, err := entry.Value.Eval(ctx)
		if err != nil {
			return nil, err
		}
		values[entry.Key] = value
	}
	return values, nil
}

type ExpressionBinder struct {
	registry FactorRegistry
}

func NewExpressionBinder(registry FactorRegistry) *ExpressionBinder {
	return &ExpressionBinder{registry: registry}
}

func (b *ExpressionBinder) Bind(expr Expr) (BoundExpr, error) {
	switch e := expr.(type) {
	case LiteralExpr:
		return BoundLiteralExpr{Value: e.Value, Type: e.Type, Field: scalarField(e.Type)}, nil
	case *LiteralExpr:
		return BoundLiteralExpr{Value: e.Value, Type: e.Type, Field: scalarField(e.Type)}, nil
	case FactorRefExpr:
		return b.bindFactorRef(e.Ref)
	case *FactorRefExpr:
		return b.bindFactorRef(e.Ref)
	case ConditionalExpr:
		return b.bindConditional(e)
	case *ConditionalExpr:
		return b.bindConditional(*e)
	case FunctionCallExpr:
		return b.bindFunctionCall(e)
	case *FunctionCallExpr:
		return b.bindFunctionCall(*e)
	case ListExpr:
		return b.bindList(e)
	case *ListExpr:
		return b.bindList(*e)
	case MapExpr:
		return b.bindMap(e)
	case *MapExpr:
		return b.bindMap(*e)
	case UnaryExpr:
		return b.bindUnary(e)
	case *UnaryExpr:
		return b.bindUnary(*e)
	case BinaryExpr:
		return b.bindBinary(e)
	case *BinaryExpr:
		return b.bindBinary(*e)
	default:
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "unsupported expression node"}
	}
}

func (b *ExpressionBinder) bindConditional(expr ConditionalExpr) (BoundExpr, error) {
	cond, err := b.Bind(expr.Cond)
	if err != nil {
		return nil, err
	}
	if cond.ResultType() != ValueTypeBool {
		return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("ternary condition requires BOOL, got %s", cond.ResultType())}
	}
	thenExpr, err := b.Bind(expr.Then)
	if err != nil {
		return nil, err
	}
	elseExpr, err := b.Bind(expr.Else)
	if err != nil {
		return nil, err
	}
	if !areBranchCompatible(thenExpr.ResultType(), elseExpr.ResultType()) {
		return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("ternary branches are incompatible: %s and %s", thenExpr.ResultType(), elseExpr.ResultType())}
	}
	resultType := mergeComparableType(thenExpr.ResultType(), elseExpr.ResultType())
	return BoundConditionalExpr{
		Cond:      cond,
		Then:      thenExpr,
		Else:      elseExpr,
		Type:      resultType,
		Field:     mergeFieldSchema(boundField(thenExpr), boundField(elseExpr), resultType),
		Evaluator: evalConditional,
	}, nil
}

func (b *ExpressionBinder) bindList(expr ListExpr) (BoundExpr, error) {
	elements := make([]BoundExpr, 0, len(expr.Elements))
	var elemType ValueType
	for idx, elem := range expr.Elements {
		boundElem, err := b.Bind(elem)
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			elemType = boundElem.ResultType()
		} else if !areEqualityCompatible(elemType, boundElem.ResultType()) {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("list element type %s is incompatible with %s", boundElem.ResultType(), elemType)}
		} else {
			elemType = mergeComparableType(elemType, boundElem.ResultType())
		}
		elements = append(elements, boundElem)
	}
	return BoundListExpr{
		Elements: elements,
		Type:     ValueTypeList,
		ElemType: elemType,
		Field: &FieldSchema{
			Type:     ValueTypeList,
			Required: true,
			ElemType: scalarField(elemType),
		},
	}, nil
}

func (b *ExpressionBinder) bindMap(expr MapExpr) (BoundExpr, error) {
	entries := make([]BoundMapEntry, 0, len(expr.Entries))
	var valueType ValueType
	for idx, entry := range expr.Entries {
		boundValue, err := b.Bind(entry.Value)
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			valueType = boundValue.ResultType()
		} else if !areEqualityCompatible(valueType, boundValue.ResultType()) {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("map value type %s is incompatible with %s", boundValue.ResultType(), valueType)}
		} else {
			valueType = mergeComparableType(valueType, boundValue.ResultType())
		}
		entries = append(entries, BoundMapEntry{
			Key:   entry.Key,
			Value: boundValue,
		})
	}
	return BoundMapExpr{
		Entries:   entries,
		Type:      ValueTypeMap,
		ValueType: valueType,
		Field: &FieldSchema{
			Type:     ValueTypeMap,
			Required: true,
			ElemType: scalarField(valueType),
		},
	}, nil
}

func (b *ExpressionBinder) bindFunctionCall(expr FunctionCallExpr) (BoundExpr, error) {
	if expr.Name == "get" {
		return b.bindGetFunction(expr)
	}

	args := make([]BoundExpr, 0, len(expr.Args))
	for _, arg := range expr.Args {
		boundArg, err := b.Bind(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, boundArg)
	}

	switch expr.Name {
	case "isEmpty":
		if len(args) != 1 {
			return nil, ValidationError{Code: ErrInvalidArgumentCount, Field: expr.Name, Message: "isEmpty expects exactly 1 argument"}
		}
		return BoundFunctionCallExpr{
			Name:      expr.Name,
			Args:      args,
			Type:      ValueTypeBool,
			Field:     scalarField(ValueTypeBool),
			Evaluator: evalFuncIsEmpty,
		}, nil
	case "exists":
		if len(args) != 1 {
			return nil, ValidationError{Code: ErrInvalidArgumentCount, Field: expr.Name, Message: "exists expects exactly 1 argument"}
		}
		return BoundFunctionCallExpr{
			Name:      expr.Name,
			Args:      args,
			Type:      ValueTypeBool,
			Field:     scalarField(ValueTypeBool),
			Evaluator: evalFuncExists,
		}, nil
	case "in":
		if len(args) < 2 {
			return nil, ValidationError{Code: ErrInvalidArgumentCount, Field: expr.Name, Message: "in expects at least 2 arguments"}
		}
		targetType := args[0].ResultType()
		for idx, arg := range args[1:] {
			if !areEqualityCompatible(targetType, arg.ResultType()) {
				return nil, ValidationError{Code: ErrTypeMismatch, Field: expr.Name, Message: fmt.Sprintf("in argument %d type %s is incompatible with target type %s", idx+2, arg.ResultType(), targetType)}
			}
		}
		return BoundFunctionCallExpr{
			Name:      expr.Name,
			Args:      args,
			Type:      ValueTypeBool,
			Field:     scalarField(ValueTypeBool),
			Evaluator: evalFuncIn,
		}, nil
	case "contains":
		if len(args) != 2 {
			return nil, ValidationError{Code: ErrInvalidArgumentCount, Field: expr.Name, Message: "contains expects exactly 2 arguments"}
		}
		switch container := args[0].(type) {
		case BoundListExpr:
			if !areEqualityCompatible(container.ElemType, args[1].ResultType()) {
				return nil, ValidationError{Code: ErrTypeMismatch, Field: expr.Name, Message: fmt.Sprintf("contains item type %s is incompatible with list element type %s", args[1].ResultType(), container.ElemType)}
			}
		case BoundFactorRefExpr:
			if container.Type != ValueTypeString {
				return nil, ValidationError{Code: ErrTypeMismatch, Field: expr.Name, Message: "contains only supports STRING or LIST container"}
			}
		default:
			if args[0].ResultType() != ValueTypeString && args[0].ResultType() != ValueTypeList {
				return nil, ValidationError{Code: ErrTypeMismatch, Field: expr.Name, Message: "contains only supports STRING or LIST container"}
			}
		}
		return BoundFunctionCallExpr{
			Name:      expr.Name,
			Args:      args,
			Type:      ValueTypeBool,
			Field:     scalarField(ValueTypeBool),
			Evaluator: evalFuncContains,
		}, nil
	case "get":
		if len(args) != 2 {
			return nil, ValidationError{Code: ErrInvalidArgumentCount, Field: expr.Name, Message: "get expects exactly 2 arguments"}
		}
		if args[1].ResultType() != ValueTypeString {
			return nil, ValidationError{Code: ErrTypeMismatch, Field: expr.Name, Message: "get key must be STRING"}
		}
		resolvedField, err := resolveGetResultField(boundField(args[0]), args[1])
		if err != nil {
			return nil, err
		}
		return BoundFunctionCallExpr{
			Name:      expr.Name,
			Args:      args,
			Type:      resolvedField.Type,
			Field:     resolvedField,
			Evaluator: evalFuncGet,
		}, nil
	default:
		return nil, ValidationError{Code: ErrUnknownFunction, Field: expr.Name, Message: "unknown function"}
	}
}

func (b *ExpressionBinder) bindGetFunction(expr FunctionCallExpr) (BoundExpr, error) {
	if len(expr.Args) != 2 {
		return nil, ValidationError{Code: ErrInvalidArgumentCount, Field: expr.Name, Message: "get expects exactly 2 arguments"}
	}

	container, err := b.bindGetContainer(expr.Args[0])
	if err != nil {
		return nil, err
	}
	keyExpr, err := b.Bind(expr.Args[1])
	if err != nil {
		return nil, err
	}
	if keyExpr.ResultType() != ValueTypeString {
		return nil, ValidationError{Code: ErrTypeMismatch, Field: expr.Name, Message: "get key must be STRING"}
	}

	resolvedField, err := resolveGetResultField(boundField(container), keyExpr)
	if err != nil {
		return nil, err
	}
	return BoundFunctionCallExpr{
		Name:      expr.Name,
		Args:      []BoundExpr{container, keyExpr},
		Type:      resolvedField.Type,
		Field:     resolvedField,
		Evaluator: evalFuncGet,
	}, nil
}

func (b *ExpressionBinder) bindGetContainer(expr Expr) (BoundExpr, error) {
	switch e := expr.(type) {
	case FactorRefExpr:
		return b.bindRootObjectFactorRefIfNeeded(e.Ref)
	case *FactorRefExpr:
		return b.bindRootObjectFactorRefIfNeeded(e.Ref)
	default:
		return b.Bind(expr)
	}
}

func (b *ExpressionBinder) bindRootObjectFactorRefIfNeeded(ref FactorRef) (BoundExpr, error) {
	if len(ref.PathSegments) > 0 {
		return b.bindFactorRef(ref)
	}

	factor, ok := b.registry[ref.FactorCode]
	if !ok {
		return nil, ValidationError{Code: ErrUnknownIdentifier, Field: ref.FactorCode, Message: "factor not found in registry"}
	}
	if schemaHasSingleValueField(factor.OutputSchema) {
		return b.bindFactorRef(ref)
	}

	return BoundFactorRefExpr{
		Ref:      ref,
		Accessor: FactorValueAccessor{FactorCode: ref.FactorCode},
		Type:     ValueTypeObject,
		Field: &FieldSchema{
			Type:     ValueTypeObject,
			Required: true,
			Fields:   factor.OutputSchema,
		},
	}, nil
}

func (b *ExpressionBinder) BindString(raw string) (Expr, BoundExpr, error) {
	ast, err := ParseExpression(raw)
	if err != nil {
		return nil, nil, err
	}
	bound, err := b.Bind(ast)
	if err != nil {
		return nil, nil, err
	}
	return ast, bound, nil
}

func (b *ExpressionBinder) bindFactorRef(ref FactorRef) (BoundExpr, error) {
	factor, ok := b.registry[ref.FactorCode]
	if !ok {
		return nil, ValidationError{Code: ErrUnknownIdentifier, Field: ref.FactorCode, Message: "factor not found in registry"}
	}
	field, err := ResolveFactorRef(ref, factor)
	if err != nil {
		return nil, err
	}
	accessor, err := CompileFactorAccessor(ref, factor)
	if err != nil {
		return nil, err
	}
	return BoundFactorRefExpr{Ref: ref, Accessor: accessor, Type: field.Type, Field: field}, nil
}

func (b *ExpressionBinder) bindUnary(expr UnaryExpr) (BoundExpr, error) {
	boundExpr, err := b.Bind(expr.Expr)
	if err != nil {
		return nil, err
	}
	switch expr.Op {
	case "!":
		if boundExpr.ResultType() != ValueTypeBool {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("operator ! requires BOOL, got %s", boundExpr.ResultType())}
		}
		return BoundUnaryExpr{Op: expr.Op, Expr: boundExpr, Type: ValueTypeBool, Field: scalarField(ValueTypeBool), Evaluator: evalNot}, nil
	case "-":
		if !isNumericType(boundExpr.ResultType()) {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("unary - requires numeric type, got %s", boundExpr.ResultType())}
		}
		return BoundUnaryExpr{Op: expr.Op, Expr: boundExpr, Type: boundExpr.ResultType(), Field: scalarField(boundExpr.ResultType()), Evaluator: evalNegate}, nil
	default:
		return nil, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported unary operator %q", expr.Op)}
	}
}

func (b *ExpressionBinder) bindBinary(expr BinaryExpr) (BoundExpr, error) {
	left, err := b.Bind(expr.Left)
	if err != nil {
		return nil, err
	}
	right, err := b.Bind(expr.Right)
	if err != nil {
		return nil, err
	}

	var resultType ValueType
	var evaluator func(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error)
	switch expr.Op {
	case "&&", "||":
		if left.ResultType() != ValueTypeBool || right.ResultType() != ValueTypeBool {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("operator %s requires BOOL operands, got %s and %s", expr.Op, left.ResultType(), right.ResultType())}
		}
		resultType = ValueTypeBool
		if expr.Op == "&&" {
			evaluator = evalAnd
		} else {
			evaluator = evalOr
		}
	case "<", "<=", ">", ">=":
		if !areComparableOrderedTypes(left.ResultType(), right.ResultType()) {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("operator %s requires comparable ordered operands, got %s and %s", expr.Op, left.ResultType(), right.ResultType())}
		}
		resultType = ValueTypeBool
		switch expr.Op {
		case "<":
			evaluator = evalLT
		case "<=":
			evaluator = evalLE
		case ">":
			evaluator = evalGT
		case ">=":
			evaluator = evalGE
		}
	case "==", "!=":
		if !areEqualityCompatible(left.ResultType(), right.ResultType()) {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("operator %s requires compatible operands, got %s and %s", expr.Op, left.ResultType(), right.ResultType())}
		}
		resultType = ValueTypeBool
		if expr.Op == "==" {
			evaluator = evalEQ
		} else {
			evaluator = evalNE
		}
	case "+", "-", "*", "/":
		if !isNumericType(left.ResultType()) || !isNumericType(right.ResultType()) {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("operator %s requires numeric operands, got %s and %s", expr.Op, left.ResultType(), right.ResultType())}
		}
		resultType = mergeNumericType(left.ResultType(), right.ResultType())
		switch expr.Op {
		case "+":
			evaluator = evalAdd
		case "-":
			evaluator = evalSub
		case "*":
			evaluator = evalMul
		case "/":
			evaluator = evalDiv
		}
	default:
		return nil, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported binary operator %q", expr.Op)}
	}

	return BoundBinaryExpr{
		Op:        expr.Op,
		Left:      left,
		Right:     right,
		Type:      resultType,
		Field:     scalarField(resultType),
		Evaluator: evaluator,
	}, nil
}

func scalarField(t ValueType) *FieldSchema {
	return &FieldSchema{Type: t, Required: true}
}

func boundField(expr BoundExpr) *FieldSchema {
	switch e := expr.(type) {
	case BoundLiteralExpr:
		return e.Field
	case BoundFactorRefExpr:
		return e.Field
	case BoundUnaryExpr:
		return e.Field
	case BoundBinaryExpr:
		return e.Field
	case BoundConditionalExpr:
		return e.Field
	case BoundFunctionCallExpr:
		return e.Field
	case BoundListExpr:
		return e.Field
	case BoundMapExpr:
		return e.Field
	default:
		return nil
	}
}

func mergeFieldSchema(left, right *FieldSchema, resultType ValueType) *FieldSchema {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left.Type == ValueTypeNull {
		return right
	}
	if right.Type == ValueTypeNull {
		return left
	}
	if resultType == ValueTypeMap && left.ElemType != nil {
		return left
	}
	if resultType == ValueTypeList && left.ElemType != nil {
		return left
	}
	if resultType == ValueTypeObject && len(left.Fields) > 0 {
		return left
	}
	return scalarField(resultType)
}

func resolveGetResultField(containerField *FieldSchema, keyExpr BoundExpr) (*FieldSchema, error) {
	if containerField == nil {
		return nil, ValidationError{Code: ErrTypeMismatch, Field: "get", Message: "get container type metadata is unavailable"}
	}
	switch containerField.Type {
	case ValueTypeMap:
		if containerField.ElemType == nil {
			return nil, ValidationError{Code: ErrTypeMismatch, Field: "get", Message: "map element type metadata is unavailable"}
		}
		return containerField.ElemType, nil
	case ValueTypeObject:
		keyLiteral, ok := keyExpr.(BoundLiteralExpr)
		if !ok || keyLiteral.Type != ValueTypeString {
			return nil, ValidationError{Code: ErrTypeMismatch, Field: "get", Message: "OBJECT get requires string literal key"}
		}
		key, _ := keyLiteral.Value.(string)
		field, ok := containerField.Fields[key]
		if !ok {
			return nil, ValidationError{Code: ErrFieldNotFound, Field: key, Message: "object field not found"}
		}
		return field, nil
	default:
		return nil, ValidationError{Code: ErrTypeMismatch, Field: "get", Message: "get only supports MAP or OBJECT container"}
	}
}
