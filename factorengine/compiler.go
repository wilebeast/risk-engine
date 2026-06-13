package factorengine

type CompiledExpr interface {
	Source() string
	AST() Expr
	Bound() BoundExpr
	Bytecode() BytecodeProgram
	ResultType() ValueType
	Eval(ctx EvalContext) (any, error)
}

type RuleCompiler interface {
	TypeCheck(expr string, registry FactorRegistry) (ValueType, error)
	Compile(expr string, registry FactorRegistry) (CompiledExpr, error)
}

type DefaultRuleCompiler struct{}

func NewRuleCompiler() RuleCompiler {
	return DefaultRuleCompiler{}
}

func (DefaultRuleCompiler) TypeCheck(expr string, registry FactorRegistry) (ValueType, error) {
	binder := NewExpressionBinder(registry)
	_, bound, err := binder.BindString(expr)
	if err != nil {
		return "", err
	}
	return bound.ResultType(), nil
}

func (DefaultRuleCompiler) Compile(expr string, registry FactorRegistry) (CompiledExpr, error) {
	binder := NewExpressionBinder(registry)
	ast, bound, err := binder.BindString(expr)
	if err != nil {
		return nil, err
	}
	bytecode, err := NewBytecodeCompiler().Compile(bound)
	if err != nil {
		return nil, err
	}

	return compiledExpr{
		source:     expr,
		ast:        ast,
		bound:      bound,
		bytecode:   bytecode,
		resultType: bound.ResultType(),
	}, nil
}

type compiledExpr struct {
	source     string
	ast        Expr
	bound      BoundExpr
	bytecode   BytecodeProgram
	resultType ValueType
}

func (c compiledExpr) Source() string {
	return c.source
}

func (c compiledExpr) AST() Expr {
	return c.ast
}

func (c compiledExpr) Bound() BoundExpr {
	return c.bound
}

func (c compiledExpr) Bytecode() BytecodeProgram {
	return c.bytecode
}

func (c compiledExpr) ResultType() ValueType {
	return c.resultType
}

func (c compiledExpr) Eval(ctx EvalContext) (any, error) {
	return NewBytecodeVM().Eval(c.bytecode, ctx)
}
