package factorengine

type EngineConfig struct {
	Registry FactorRegistry
	Compiler RuleCompiler
	Cache    ProgramCache
	Observer RuleObserver
	VMConfig BytecodeVMConfig
}

type Engine struct {
	registry FactorRegistry
	compiler RuleCompiler
	observer RuleObserver
	vmConfig BytecodeVMConfig
}

func NewEngine(config EngineConfig) *Engine {
	observer := config.Observer
	if observer == nil {
		observer = NoopRuleObserver{}
	}

	compiler := config.Compiler
	if compiler == nil {
		compiler = NewRuleCompiler()
	}
	if config.Cache != nil {
		compiler = NewObservedCachedRuleCompiler(compiler, config.Cache, observer)
	}
	compiler = NewObservedRuleCompiler(compiler, observer)

	return &Engine{
		registry: config.Registry,
		compiler: compiler,
		observer: observer,
		vmConfig: config.VMConfig,
	}
}

func (e *Engine) Registry() FactorRegistry {
	return e.registry
}

func (e *Engine) TypeCheck(expr string) (ValueType, error) {
	return e.compiler.TypeCheck(expr, e.registry)
}

func (e *Engine) Compile(expr string) (CompiledExpr, error) {
	compiled, err := e.compiler.Compile(expr, e.registry)
	if err != nil {
		return nil, err
	}
	return engineCompiledExpr{
		CompiledExpr: compiled,
		observer:     e.observer,
		vmConfig:     e.vmConfig,
	}, nil
}

func (e *Engine) Eval(expr string, ctx EvalContext) (any, error) {
	compiled, err := e.Compile(expr)
	if err != nil {
		return nil, err
	}
	return compiled.Eval(ctx)
}

type engineCompiledExpr struct {
	CompiledExpr
	observer RuleObserver
	vmConfig BytecodeVMConfig
}

func (c engineCompiledExpr) Eval(ctx EvalContext) (any, error) {
	vm := NewBytecodeVMWithConfig(c.vmConfig)
	result, err := vm.Eval(c.Bytecode(), ctx)
	return result, err
}
