package factorengine

import (
	"sync/atomic"
	"time"
)

type CompileObservation struct {
	Expr        string
	Fingerprint string
	Duration    time.Duration
	Err         error
}

type EvalObservation struct {
	Expr        string
	Fingerprint string
	Duration    time.Duration
	Err         error
}

type CacheObservation struct {
	Expr        string
	Key         string
	Fingerprint string
	Hit         bool
}

type RuleObserver interface {
	ObserveCompile(CompileObservation)
	ObserveEval(EvalObservation)
	ObserveCache(CacheObservation)
}

type NoopRuleObserver struct{}

func (NoopRuleObserver) ObserveCompile(CompileObservation) {}

func (NoopRuleObserver) ObserveEval(EvalObservation) {}

func (NoopRuleObserver) ObserveCache(CacheObservation) {}

type RuleObserverStats struct {
	CompileCount         uint64
	CompileErrors        uint64
	CompileDurationNanos uint64
	EvalCount            uint64
	EvalErrors           uint64
	EvalDurationNanos    uint64
	CacheRequests        uint64
	CacheHits            uint64
	CacheMisses          uint64
}

type InMemoryRuleObserver struct {
	compileCount         atomic.Uint64
	compileErrors        atomic.Uint64
	compileDurationNanos atomic.Uint64
	evalCount            atomic.Uint64
	evalErrors           atomic.Uint64
	evalDurationNanos    atomic.Uint64
	cacheRequests        atomic.Uint64
	cacheHits            atomic.Uint64
	cacheMisses          atomic.Uint64
}

func NewInMemoryRuleObserver() *InMemoryRuleObserver {
	return &InMemoryRuleObserver{}
}

func (o *InMemoryRuleObserver) ObserveCompile(obs CompileObservation) {
	o.compileCount.Add(1)
	o.compileDurationNanos.Add(uint64(obs.Duration))
	if obs.Err != nil {
		o.compileErrors.Add(1)
	}
}

func (o *InMemoryRuleObserver) ObserveEval(obs EvalObservation) {
	o.evalCount.Add(1)
	o.evalDurationNanos.Add(uint64(obs.Duration))
	if obs.Err != nil {
		o.evalErrors.Add(1)
	}
}

func (o *InMemoryRuleObserver) ObserveCache(obs CacheObservation) {
	o.cacheRequests.Add(1)
	if obs.Hit {
		o.cacheHits.Add(1)
	} else {
		o.cacheMisses.Add(1)
	}
}

func (o *InMemoryRuleObserver) Stats() RuleObserverStats {
	return RuleObserverStats{
		CompileCount:         o.compileCount.Load(),
		CompileErrors:        o.compileErrors.Load(),
		CompileDurationNanos: o.compileDurationNanos.Load(),
		EvalCount:            o.evalCount.Load(),
		EvalErrors:           o.evalErrors.Load(),
		EvalDurationNanos:    o.evalDurationNanos.Load(),
		CacheRequests:        o.cacheRequests.Load(),
		CacheHits:            o.cacheHits.Load(),
		CacheMisses:          o.cacheMisses.Load(),
	}
}

type observedRuleCompiler struct {
	inner    RuleCompiler
	observer RuleObserver
}

func NewObservedRuleCompiler(inner RuleCompiler, observer RuleObserver) RuleCompiler {
	if inner == nil {
		inner = NewRuleCompiler()
	}
	if observer == nil {
		observer = NoopRuleObserver{}
	}
	return observedRuleCompiler{
		inner:    inner,
		observer: observer,
	}
}

func (c observedRuleCompiler) TypeCheck(expr string, registry FactorRegistry) (ValueType, error) {
	return c.inner.TypeCheck(expr, registry)
}

func (c observedRuleCompiler) Compile(expr string, registry FactorRegistry) (CompiledExpr, error) {
	start := time.Now()
	compiled, err := c.inner.Compile(expr, registry)
	duration := time.Since(start)

	observation := CompileObservation{
		Expr:     expr,
		Duration: duration,
		Err:      err,
	}
	if err == nil {
		observation.Fingerprint = compiled.Fingerprint()
		compiled = observedCompiledExpr{
			CompiledExpr: compiled,
			observer:     c.observer,
		}
	}
	c.observer.ObserveCompile(observation)
	return compiled, err
}

type observedCompiledExpr struct {
	CompiledExpr
	observer RuleObserver
}

func (c observedCompiledExpr) Eval(ctx EvalContext) (any, error) {
	start := time.Now()
	result, err := c.CompiledExpr.Eval(ctx)
	c.observer.ObserveEval(EvalObservation{
		Expr:        c.Source(),
		Fingerprint: c.Fingerprint(),
		Duration:    time.Since(start),
		Err:         err,
	})
	return result, err
}
