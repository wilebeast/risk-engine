package factorengine

import (
	"fmt"
	"strings"
)

type EvalContext map[string]any

func ResolveValue(ctx EvalContext, ref FactorRef) (any, bool) {
	root, ok := ctx[ref.FactorCode]
	if !ok {
		return nil, false
	}

	path := ref.PathSegments
	if len(path) == 0 {
		if obj, ok := root.(map[string]any); ok {
			if value, ok := obj["value"]; ok {
				return value, true
			}
		}
		return root, true
	}

	current := root
	for _, segment := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := obj[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func CompareLessThan(left any, right any) bool {
	switch lv := left.(type) {
	case int:
		rv, ok := right.(int)
		return ok && lv < rv
	case int64:
		rv, ok := right.(int64)
		return ok && lv < rv
	case float64:
		rv, ok := right.(float64)
		return ok && lv < rv
	default:
		return false
	}
}

func EvalExpr(expr Expr, ctx EvalContext) (any, error) {
	switch e := expr.(type) {
	case LiteralExpr:
		return e.Value, nil
	case *LiteralExpr:
		return e.Value, nil
	case FactorRefExpr:
		return evalFactorRef(e.Ref, ctx), nil
	case *FactorRefExpr:
		return evalFactorRef(e.Ref, ctx), nil
	case ConditionalExpr:
		return evalConditionalExpr(e, ctx)
	case *ConditionalExpr:
		return evalConditionalExpr(*e, ctx)
	case FunctionCallExpr:
		return evalFunctionCall(e, ctx)
	case *FunctionCallExpr:
		return evalFunctionCall(*e, ctx)
	case ListExpr:
		return evalListExpr(e, ctx)
	case *ListExpr:
		return evalListExpr(*e, ctx)
	case MapExpr:
		return evalMapExpr(e, ctx)
	case *MapExpr:
		return evalMapExpr(*e, ctx)
	case UnaryExpr:
		return evalUnary(e, ctx)
	case *UnaryExpr:
		return evalUnary(*e, ctx)
	case BinaryExpr:
		return evalBinary(e, ctx)
	case *BinaryExpr:
		return evalBinary(*e, ctx)
	default:
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "unsupported expression node"}
	}
}

func evalConditionalExpr(expr ConditionalExpr, ctx EvalContext) (any, error) {
	cond, err := EvalExpr(expr.Cond, ctx)
	if err != nil {
		return nil, err
	}
	if truthyBool(cond) {
		return EvalExpr(expr.Then, ctx)
	}
	return EvalExpr(expr.Else, ctx)
}

func evalListExpr(expr ListExpr, ctx EvalContext) (any, error) {
	values := make([]any, 0, len(expr.Elements))
	for _, elem := range expr.Elements {
		value, err := EvalExpr(elem, ctx)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func evalMapExpr(expr MapExpr, ctx EvalContext) (any, error) {
	values := make(map[string]any, len(expr.Entries))
	for _, entry := range expr.Entries {
		value, err := EvalExpr(entry.Value, ctx)
		if err != nil {
			return nil, err
		}
		values[entry.Key] = value
	}
	return values, nil
}

func evalFunctionCall(expr FunctionCallExpr, ctx EvalContext) (any, error) {
	boundArgs := make([]BoundExpr, 0, len(expr.Args))
	for _, arg := range expr.Args {
		boundArgs = append(boundArgs, exprBoundAdapter{expr: arg})
	}
	switch expr.Name {
	case "isEmpty":
		return evalFuncIsEmpty(boundArgs, ctx)
	case "exists":
		return evalFuncExists(boundArgs, ctx)
	case "in":
		return evalFuncIn(boundArgs, ctx)
	case "contains":
		return evalFuncContains(boundArgs, ctx)
	case "get":
		return evalFuncGet(boundArgs, ctx)
	default:
		return nil, ValidationError{Code: ErrUnknownFunction, Field: expr.Name, Message: "unknown function"}
	}
}

func evalFactorRef(ref FactorRef, ctx EvalContext) any {
	value, ok := ResolveValue(ctx, ref)
	if !ok {
		return nil
	}
	return value
}

func evalUnary(expr UnaryExpr, ctx EvalContext) (any, error) {
	value, err := EvalExpr(expr.Expr, ctx)
	if err != nil {
		return nil, err
	}
	return evalUnaryValue(expr.Op, value)
}

func evalUnaryValue(op string, value any) (any, error) {
	switch op {
	case "!":
		if value == nil {
			return false, nil
		}
		boolValue, ok := value.(bool)
		if !ok {
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("operator ! requires BOOL, got %T", value)}
		}
		return !boolValue, nil
	case "-":
		if value == nil {
			return nil, nil
		}
		switch v := value.(type) {
		case int:
			return -v, nil
		case int64:
			return -v, nil
		case float64:
			return -v, nil
		default:
			return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("unary - requires numeric type, got %T", value)}
		}
	default:
		return nil, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported unary operator %q", op)}
	}
}

func evalBinary(expr BinaryExpr, ctx EvalContext) (any, error) {
	return evalBinaryBound(BoundBinaryExpr{
		Op:        expr.Op,
		Left:      exprBoundAdapter{expr.Left},
		Right:     exprBoundAdapter{expr.Right},
		Evaluator: evalBinaryOp(expr.Op),
	}, ctx)
}

func evalBinaryBound(expr BoundBinaryExpr, ctx EvalContext) (any, error) {
	return expr.Evaluator(expr.Left, expr.Right, ctx)
}

func evalConditional(cond BoundExpr, thenExpr BoundExpr, elseExpr BoundExpr, ctx EvalContext) (any, error) {
	condValue, err := cond.Eval(ctx)
	if err != nil {
		return nil, err
	}
	if truthyBool(condValue) {
		return thenExpr.Eval(ctx)
	}
	return elseExpr.Eval(ctx)
}

type exprBoundAdapter struct {
	expr Expr
}

func (exprBoundAdapter) boundNode() {}

func (e exprBoundAdapter) ResultType() ValueType {
	return ""
}

func (e exprBoundAdapter) Eval(ctx EvalContext) (any, error) {
	return EvalExpr(e.expr, ctx)
}

func truthyBool(value any) bool {
	boolValue, ok := value.(bool)
	return ok && boolValue
}

func evalNot(value any) (any, error) {
	return evalUnaryValue("!", value)
}

func evalNegate(value any) (any, error) {
	return evalUnaryValue("-", value)
}

func evalAnd(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	leftValue, err := left.Eval(ctx)
	if err != nil {
		return nil, err
	}
	if !truthyBool(leftValue) {
		return false, nil
	}
	rightValue, err := right.Eval(ctx)
	if err != nil {
		return nil, err
	}
	return truthyBool(rightValue), nil
}

func evalOr(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	leftValue, err := left.Eval(ctx)
	if err != nil {
		return nil, err
	}
	if truthyBool(leftValue) {
		return true, nil
	}
	rightValue, err := right.Eval(ctx)
	if err != nil {
		return nil, err
	}
	return truthyBool(rightValue), nil
}

func evalLT(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalOrdered(left, right, ctx, "<")
}

func evalLE(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalOrdered(left, right, ctx, "<=")
}

func evalGT(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalOrdered(left, right, ctx, ">")
}

func evalGE(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalOrdered(left, right, ctx, ">=")
}

func evalEQ(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	leftValue, rightValue, err := evalOperands(left, right, ctx)
	if err != nil {
		return nil, err
	}
	return equalityCompare(leftValue, rightValue), nil
}

func evalNE(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	leftValue, rightValue, err := evalOperands(left, right, ctx)
	if err != nil {
		return nil, err
	}
	return !equalityCompare(leftValue, rightValue), nil
}

func evalAdd(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalArithmetic(left, right, ctx, "+")
}

func evalSub(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalArithmetic(left, right, ctx, "-")
}

func evalMul(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalArithmetic(left, right, ctx, "*")
}

func evalDiv(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	return evalArithmetic(left, right, ctx, "/")
}

func evalOrdered(left BoundExpr, right BoundExpr, ctx EvalContext, op string) (any, error) {
	leftValue, rightValue, err := evalOperands(left, right, ctx)
	if err != nil {
		return nil, err
	}
	return orderedCompare(leftValue, rightValue, op)
}

func evalArithmetic(left BoundExpr, right BoundExpr, ctx EvalContext, op string) (any, error) {
	leftValue, rightValue, err := evalOperands(left, right, ctx)
	if err != nil {
		return nil, err
	}
	return arithmetic(leftValue, rightValue, op)
}

func evalOperands(left BoundExpr, right BoundExpr, ctx EvalContext) (any, any, error) {
	leftValue, err := left.Eval(ctx)
	if err != nil {
		return nil, nil, err
	}
	rightValue, err := right.Eval(ctx)
	if err != nil {
		return nil, nil, err
	}
	return leftValue, rightValue, nil
}

func evalBinaryOp(op string) func(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
	switch op {
	case "&&":
		return evalAnd
	case "||":
		return evalOr
	case "<":
		return evalLT
	case "<=":
		return evalLE
	case ">":
		return evalGT
	case ">=":
		return evalGE
	case "==":
		return evalEQ
	case "!=":
		return evalNE
	case "+":
		return evalAdd
	case "-":
		return evalSub
	case "*":
		return evalMul
	case "/":
		return evalDiv
	default:
		return func(left BoundExpr, right BoundExpr, ctx EvalContext) (any, error) {
			return nil, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported binary operator %q", op)}
		}
	}
}

func evalFuncIsEmpty(args []BoundExpr, ctx EvalContext) (any, error) {
	values, err := evalBoundArgs(args, ctx)
	if err != nil {
		return nil, err
	}
	return builtinIsEmpty(values)
}

func evalFuncExists(args []BoundExpr, ctx EvalContext) (any, error) {
	values, err := evalBoundArgs(args, ctx)
	if err != nil {
		return nil, err
	}
	return builtinExists(values)
}

func evalFuncIn(args []BoundExpr, ctx EvalContext) (any, error) {
	values, err := evalBoundArgs(args, ctx)
	if err != nil {
		return nil, err
	}
	return builtinIn(values)
}

func evalFuncContains(args []BoundExpr, ctx EvalContext) (any, error) {
	values, err := evalBoundArgs(args, ctx)
	if err != nil {
		return nil, err
	}
	return builtinContains(values)
}

func evalFuncGet(args []BoundExpr, ctx EvalContext) (any, error) {
	values, err := evalBoundArgs(args, ctx)
	if err != nil {
		return nil, err
	}
	return builtinGet(values)
}

func evalBoundArgs(args []BoundExpr, ctx EvalContext) ([]any, error) {
	values := make([]any, 0, len(args))
	for _, arg := range args {
		value, err := arg.Eval(ctx)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func builtinIsEmpty(args []any) (any, error) {
	value := args[0]
	if value == nil {
		return true, nil
	}
	switch v := value.(type) {
	case string:
		return v == "", nil
	case []any:
		return len(v) == 0, nil
	case map[string]any:
		return len(v) == 0, nil
	default:
		return false, nil
	}
}

func builtinExists(args []any) (any, error) {
	return args[0] != nil, nil
}

func builtinIn(args []any) (any, error) {
	target := args[0]
	for _, candidate := range args[1:] {
		if equalityCompare(target, candidate) {
			return true, nil
		}
	}
	return false, nil
}

func builtinContains(args []any) (any, error) {
	container := args[0]
	item := args[1]
	switch v := container.(type) {
	case string:
		itemStr, ok := item.(string)
		if !ok {
			return false, nil
		}
		return strings.Contains(v, itemStr), nil
	case []any:
		for _, candidate := range v {
			if equalityCompare(candidate, item) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

func builtinGet(args []any) (any, error) {
	container := args[0]
	keyValue := args[1]
	key, ok := keyValue.(string)
	if !ok {
		return nil, ValidationError{Code: ErrTypeMismatch, Message: "get key must evaluate to STRING"}
	}
	obj, ok := container.(map[string]any)
	if !ok {
		return nil, ValidationError{Code: ErrTypeMismatch, Message: "get container must evaluate to MAP or OBJECT"}
	}
	value, ok := obj[key]
	if !ok {
		return nil, nil
	}
	return value, nil
}

func orderedCompare(left, right any, op string) (bool, error) {
	if left == nil || right == nil {
		return false, nil
	}
	if leftString, ok := left.(string); ok {
		rightString, ok := right.(string)
		if !ok {
			return false, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("ordered compare requires comparable values, got %T and %T", left, right)}
		}
		switch op {
		case "<":
			return leftString < rightString, nil
		case "<=":
			return leftString <= rightString, nil
		case ">":
			return leftString > rightString, nil
		case ">=":
			return leftString >= rightString, nil
		}
	}

	leftNum, rightNum, kind, ok := normalizeNumbers(left, right)
	if !ok {
		return false, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("ordered compare requires numeric or string values, got %T and %T", left, right)}
	}
	switch kind {
	case ValueTypeDouble:
		switch op {
		case "<":
			return leftNum.float64 < rightNum.float64, nil
		case "<=":
			return leftNum.float64 <= rightNum.float64, nil
		case ">":
			return leftNum.float64 > rightNum.float64, nil
		case ">=":
			return leftNum.float64 >= rightNum.float64, nil
		}
	case ValueTypeInt:
		switch op {
		case "<":
			return leftNum.int64 < rightNum.int64, nil
		case "<=":
			return leftNum.int64 <= rightNum.int64, nil
		case ">":
			return leftNum.int64 > rightNum.int64, nil
		case ">=":
			return leftNum.int64 >= rightNum.int64, nil
		}
	case ValueTypeLong:
		switch op {
		case "<":
			return leftNum.int64 < rightNum.int64, nil
		case "<=":
			return leftNum.int64 <= rightNum.int64, nil
		case ">":
			return leftNum.int64 > rightNum.int64, nil
		case ">=":
			return leftNum.int64 >= rightNum.int64, nil
		}
	}
	return false, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported ordered operator %q", op)}
}

func equalityCompare(left, right any) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	if leftBool, ok := left.(bool); ok {
		rightBool, ok := right.(bool)
		return ok && leftBool == rightBool
	}
	if leftString, ok := left.(string); ok {
		rightString, ok := right.(string)
		return ok && leftString == rightString
	}
	leftNum, rightNum, kind, ok := normalizeNumbers(left, right)
	if !ok {
		return false
	}
	if kind == ValueTypeDouble {
		return leftNum.float64 == rightNum.float64
	}
	return leftNum.int64 == rightNum.int64
}

func arithmetic(left, right any, op string) (any, error) {
	if left == nil || right == nil {
		return nil, nil
	}
	leftNum, rightNum, kind, ok := normalizeNumbers(left, right)
	if !ok {
		return nil, ValidationError{Code: ErrTypeMismatch, Message: fmt.Sprintf("arithmetic requires numeric values, got %T and %T", left, right)}
	}
	if kind == ValueTypeDouble {
		switch op {
		case "+":
			return leftNum.float64 + rightNum.float64, nil
		case "-":
			return leftNum.float64 - rightNum.float64, nil
		case "*":
			return leftNum.float64 * rightNum.float64, nil
		case "/":
			if rightNum.float64 == 0 {
				return nil, ValidationError{Code: ErrDivisionByZero, Message: "division by zero"}
			}
			return leftNum.float64 / rightNum.float64, nil
		}
	}
	switch op {
	case "+":
		return leftNum.int64 + rightNum.int64, nil
	case "-":
		return leftNum.int64 - rightNum.int64, nil
	case "*":
		return leftNum.int64 * rightNum.int64, nil
	case "/":
		if rightNum.int64 == 0 {
			return nil, ValidationError{Code: ErrDivisionByZero, Message: "division by zero"}
		}
		return leftNum.int64 / rightNum.int64, nil
	}
	return nil, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported arithmetic operator %q", op)}
}

type normalizedNumber struct {
	int64   int64
	float64 float64
}

func normalizeNumbers(left, right any) (normalizedNumber, normalizedNumber, ValueType, bool) {
	lt, lv, lok := numberValue(left)
	rt, rv, rok := numberValue(right)
	if !lok || !rok {
		return normalizedNumber{}, normalizedNumber{}, "", false
	}
	resultType := mergeNumericType(lt, rt)
	if resultType == ValueTypeDouble {
		return normalizedNumber{float64: toFloat64(lt, lv)}, normalizedNumber{float64: toFloat64(rt, rv)}, resultType, true
	}
	return normalizedNumber{int64: toInt64(lt, lv)}, normalizedNumber{int64: toInt64(rt, rv)}, resultType, true
}

func numberValue(value any) (ValueType, any, bool) {
	switch v := value.(type) {
	case int:
		return ValueTypeInt, v, true
	case int64:
		return ValueTypeLong, v, true
	case float64:
		return ValueTypeDouble, v, true
	default:
		return "", nil, false
	}
}

func toFloat64(t ValueType, value any) float64 {
	switch t {
	case ValueTypeInt:
		return float64(value.(int))
	case ValueTypeLong:
		return float64(value.(int64))
	case ValueTypeDouble:
		return value.(float64)
	default:
		return 0
	}
}

func toInt64(t ValueType, value any) int64 {
	switch t {
	case ValueTypeInt:
		return int64(value.(int))
	case ValueTypeLong:
		return value.(int64)
	default:
		return 0
	}
}
