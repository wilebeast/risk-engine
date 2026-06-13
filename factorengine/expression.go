package factorengine

type Expr interface {
	exprNode()
}

type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (BinaryExpr) exprNode() {}

type ConditionalExpr struct {
	Cond Expr
	Then Expr
	Else Expr
}

func (ConditionalExpr) exprNode() {}

type UnaryExpr struct {
	Op   string
	Expr Expr
}

func (UnaryExpr) exprNode() {}

type FactorRefExpr struct {
	Ref FactorRef
}

func (FactorRefExpr) exprNode() {}

type FunctionCallExpr struct {
	Name string
	Args []Expr
}

func (FunctionCallExpr) exprNode() {}

type ListExpr struct {
	Elements []Expr
}

func (ListExpr) exprNode() {}

type MapEntryExpr struct {
	Key   string
	Value Expr
}

type MapExpr struct {
	Entries []MapEntryExpr
}

func (MapExpr) exprNode() {}

type LiteralExpr struct {
	Value any
	Type  ValueType
}

func (LiteralExpr) exprNode() {}

type ExpressionTypeChecker struct {
	binder *ExpressionBinder
}

func NewExpressionTypeChecker(registry FactorRegistry) *ExpressionTypeChecker {
	return &ExpressionTypeChecker{binder: NewExpressionBinder(registry)}
}

func (c *ExpressionTypeChecker) Check(expr Expr) (ValueType, error) {
	bound, err := c.binder.Bind(expr)
	if err != nil {
		return "", err
	}
	return bound.ResultType(), nil
}

func (c *ExpressionTypeChecker) CheckString(raw string) (ValueType, error) {
	expr, err := ParseExpression(raw)
	if err != nil {
		return "", err
	}
	return c.Check(expr)
}
