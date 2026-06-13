package factorengine

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

type panicAccessor struct{}

func (panicAccessor) Get(EvalContext) (any, bool) {
	panic("accessor boom")
}

type countingRuleCompiler struct {
	compileCalls int
	result       CompiledExpr
	err          error
}

func (c *countingRuleCompiler) TypeCheck(expr string, registry FactorRegistry) (ValueType, error) {
	return ValueTypeBool, nil
}

func (c *countingRuleCompiler) Compile(expr string, registry FactorRegistry) (CompiledExpr, error) {
	c.compileCalls++
	return c.result, c.err
}

func TestParseFactorRef(t *testing.T) {
	ref, err := ParseFactorRef("f_user_info.register_days")
	if err != nil {
		t.Fatalf("ParseFactorRef returned error: %v", err)
	}
	if ref.FactorCode != "f_user_info" {
		t.Fatalf("unexpected factor code: %s", ref.FactorCode)
	}
	if len(ref.PathSegments) != 1 || ref.PathSegments[0] != "register_days" {
		t.Fatalf("unexpected path: %#v", ref.PathSegments)
	}
}

func TestResolveFactorRefSingleValueFactorAllowsBareRef(t *testing.T) {
	factor := &FactorDefinition{
		FactorCode: "f_user_id",
		OutputSchema: Schema{
			"value": {Type: ValueTypeString, Required: true},
		},
	}
	field, err := ResolveFactorRef(FactorRef{FactorCode: "f_user_id"}, factor)
	if err != nil {
		t.Fatalf("ResolveFactorRef returned error: %v", err)
	}
	if field.Type != ValueTypeString {
		t.Fatalf("unexpected field type: %s", field.Type)
	}
}

func TestResolveFactorRefMultiFieldRequiresExplicitPath(t *testing.T) {
	factor := &FactorDefinition{
		FactorCode: "f_user_info",
		OutputSchema: Schema{
			"register_days": {Type: ValueTypeInt, Required: true},
			"kyc_level":     {Type: ValueTypeString},
		},
	}
	_, err := ResolveFactorRef(FactorRef{FactorCode: "f_user_info"}, factor)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrFactorRefAmbiguous {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveFactorRefNestedPath(t *testing.T) {
	factor := &FactorDefinition{
		FactorCode: "f_device_info",
		OutputSchema: Schema{
			"risk": {
				Type: ValueTypeObject,
				Fields: map[string]*FieldSchema{
					"level": {Type: ValueTypeString, Required: true},
				},
			},
		},
	}
	field, err := ResolveFactorRef(FactorRef{FactorCode: "f_device_info", PathSegments: []string{"risk", "level"}}, factor)
	if err != nil {
		t.Fatalf("ResolveFactorRef returned error: %v", err)
	}
	if field.Type != ValueTypeString {
		t.Fatalf("unexpected field type: %s", field.Type)
	}
}

func TestValidateRPCFactorSuccess(t *testing.T) {
	registry := FactorRegistry{
		"f_user_id": {
			FactorCode: "f_user_id",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
			Published: true,
		},
	}
	_jsii := NewMemoryIDLRegistry()
	_jsii.Register("user_profile", 3, "UserProfileService", "GetUserProfile", &ThriftMethod{
		Request: map[string]*TypeDescriptor{
			"user_id": {Type: ValueTypeString, Required: true},
		},
		Response: map[string]*TypeDescriptor{
			"register_days": {Type: ValueTypeInt, Required: true},
			"kyc_level":     {Type: ValueTypeString, Required: false},
			"profile": {
				Type:     ValueTypeObject,
				Required: false,
				Fields: map[string]*TypeDescriptor{
					"account_status": {Type: ValueTypeString, Required: false},
				},
			},
		},
	})

	def := &FactorDefinition{
		FactorCode:     "f_user_info",
		FactorCategory: FactorCategoryRPC,
		FactorType:     FactorTypeThriftInvoker,
		InputSchema: Schema{
			"user_id": {Type: ValueTypeString, Required: true},
		},
		OutputSchema: Schema{
			"register_days":  {Type: ValueTypeInt, Required: true},
			"kyc_level":      {Type: ValueTypeString, Required: false},
			"account_status": {Type: ValueTypeString, Required: false},
		},
		Dependencies: []string{"f_user_id"},
		Config: RPCFactorConfig{
			IDLCode:    "user_profile",
			IDLVersion: 3,
			Service:    "UserProfileService",
			Method:     "GetUserProfile",
			RequestMapping: map[string]string{
				"user_id": "f_user_id",
			},
			ResponseMapping: map[string]string{
				"register_days":  "register_days",
				"kyc_level":      "kyc_level",
				"account_status": "profile.account_status",
			},
		},
	}

	errs := ValidateRPCFactor(def, registry, _jsii)
	if len(errs) != 0 {
		t.Fatalf("expected success, got errors: %v", errs)
	}
}

func TestValidateRPCFactorWithRealThriftFile(t *testing.T) {
	registry := FactorRegistry{
		"f_user_id": {
			FactorCode: "f_user_id",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
			Published: true,
		},
	}

	_jsii := NewMemoryIDLRegistry()
	thriftPath := filepath.Join("testdata", "user_profile.thrift")
	if err := _jsii.RegisterThriftFile("user_profile", 3, thriftPath); err != nil {
		t.Fatalf("RegisterThriftFile returned error: %v", err)
	}

	def := &FactorDefinition{
		FactorCode:     "f_user_info",
		FactorCategory: FactorCategoryRPC,
		FactorType:     FactorTypeThriftInvoker,
		InputSchema: Schema{
			"user_id": {Type: ValueTypeString, Required: true},
		},
		OutputSchema: Schema{
			"register_days":  {Type: ValueTypeInt, Required: true},
			"kyc_level":      {Type: ValueTypeString, Required: false},
			"account_status": {Type: ValueTypeString, Required: false},
		},
		Dependencies: []string{"f_user_id"},
		Config: RPCFactorConfig{
			IDLCode:    "user_profile",
			IDLVersion: 3,
			Service:    "UserProfileService",
			Method:     "GetUserProfile",
			RequestMapping: map[string]string{
				"user_id": "f_user_id",
			},
			ResponseMapping: map[string]string{
				"register_days":  "register_days",
				"kyc_level":      "kyc_level",
				"account_status": "profile.account_status",
			},
		},
	}

	errs := ValidateRPCFactor(def, registry, _jsii)
	if len(errs) != 0 {
		t.Fatalf("expected success with thrift file, got errors: %v", errs)
	}
}

func TestValidateRPCFactorFailures(t *testing.T) {
	registry := FactorRegistry{
		"f_user_id": {
			FactorCode: "f_user_id",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
			Published: false,
		},
	}
	_jsii := NewMemoryIDLRegistry()
	_jsii.Register("user_profile", 3, "UserProfileService", "GetUserProfile", &ThriftMethod{
		Request: map[string]*TypeDescriptor{
			"user_id": {Type: ValueTypeString, Required: true},
		},
		Response: map[string]*TypeDescriptor{
			"register_days": {Type: ValueTypeString, Required: false},
		},
	})

	def := &FactorDefinition{
		FactorCode:     "f_user_info",
		FactorCategory: FactorCategoryRPC,
		FactorType:     FactorTypeThriftInvoker,
		InputSchema: Schema{
			"user_id": {Type: ValueTypeString, Required: true},
		},
		OutputSchema: Schema{
			"register_days": {Type: ValueTypeInt, Required: true},
		},
		Dependencies: []string{},
		Config: RPCFactorConfig{
			IDLCode:    "user_profile",
			IDLVersion: 3,
			Service:    "UserProfileService",
			Method:     "GetUserProfile",
			RequestMapping: map[string]string{
				"user_id": "f_user_id",
			},
			ResponseMapping: map[string]string{
				"register_days": "register_days",
			},
		},
	}

	errs := ValidateRPCFactor(def, registry, _jsii)
	if len(errs) < 4 {
		t.Fatalf("expected multiple errors, got: %v", errs)
	}
}

func TestResolveValueAndNullSemantics(t *testing.T) {
	ctx := EvalContext{
		"f_user_info": map[string]any{
			"register_days": 3,
		},
		"f_user_id": map[string]any{
			"value": "u_1",
		},
	}

	value, ok := ResolveValue(ctx, FactorRef{FactorCode: "f_user_info", PathSegments: []string{"register_days"}})
	if !ok || value.(int) != 3 {
		t.Fatalf("unexpected resolved value: %v, %v", value, ok)
	}

	if !CompareLessThan(value, 7) {
		t.Fatal("expected comparison to be true")
	}

	missing, ok := ResolveValue(ctx, FactorRef{FactorCode: "f_user_info", PathSegments: []string{"kyc_level"}})
	if ok || missing != nil {
		t.Fatalf("expected missing field to resolve to null semantics, got %v, %v", missing, ok)
	}

	bare, ok := ResolveValue(ctx, FactorRef{FactorCode: "f_user_id"})
	if !ok || bare.(string) != "u_1" {
		t.Fatalf("expected default value resolution, got %v, %v", bare, ok)
	}

	if CompareLessThan(nil, 7) {
		t.Fatal("nil comparison should be false")
	}
}

func TestExpressionTypeCheckerSuccess(t *testing.T) {
	registry := FactorRegistry{
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
			},
		},
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}

	checker := NewExpressionTypeChecker(registry)
	valueType, err := checker.CheckString("f_amount > 100000 && f_user_info.register_days < 7 && f_is_vip")
	if err != nil {
		t.Fatalf("CheckString returned error: %v", err)
	}
	if valueType != ValueTypeBool {
		t.Fatalf("unexpected expression type: %s", valueType)
	}
}

func TestExpressionTypeCheckerTypeMismatch(t *testing.T) {
	registry := FactorRegistry{
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
			},
		},
	}

	checker := NewExpressionTypeChecker(registry)
	_, err := checker.CheckString("f_user_info.register_days && true")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpressionTypeCheckerRejectsObjectFactor(t *testing.T) {
	registry := FactorRegistry{
		"f_device_info": {
			FactorCode: "f_device_info",
			OutputSchema: Schema{
				"risk": {
					Type: ValueTypeObject,
					Fields: map[string]*FieldSchema{
						"level": {Type: ValueTypeString, Required: true},
					},
				},
			},
		},
	}

	checker := NewExpressionTypeChecker(registry)
	_, err := checker.CheckString("f_device_info.risk == \"high\"")
	if err == nil {
		t.Fatal("expected object type error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseExpression(t *testing.T) {
	expr, err := ParseExpression("(f_user_info.register_days < 7 && true) || f_amount > 100")
	if err != nil {
		t.Fatalf("ParseExpression returned error: %v", err)
	}
	if _, ok := expr.(BinaryExpr); !ok {
		t.Fatalf("expected root binary expression, got %T", expr)
	}
}

func TestRuleCompilerTypeCheckAndCompile(t *testing.T) {
	registry := FactorRegistry{
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
			},
		},
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}

	compiler := NewRuleCompiler()
	expr := "f_amount > 100000 && f_user_info.register_days < 7"

	valueType, err := compiler.TypeCheck(expr, registry)
	if err != nil {
		t.Fatalf("TypeCheck returned error: %v", err)
	}
	if valueType != ValueTypeBool {
		t.Fatalf("unexpected typecheck result: %s", valueType)
	}

	compiled, err := compiler.Compile(expr, registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if compiled.Source() != expr {
		t.Fatalf("unexpected compiled source: %s", compiled.Source())
	}
	if compiled.ResultType() != ValueTypeBool {
		t.Fatalf("unexpected compiled result type: %s", compiled.ResultType())
	}
	if _, ok := compiled.AST().(BinaryExpr); !ok {
		t.Fatalf("expected binary AST root, got %T", compiled.AST())
	}
	if compiled.Bound() == nil {
		t.Fatal("expected compiled bound tree")
	}
	if compiled.Bound().ResultType() != ValueTypeBool {
		t.Fatalf("unexpected bound result type: %s", compiled.Bound().ResultType())
	}
	if len(compiled.Bytecode().Instructions) == 0 {
		t.Fatal("expected compiled bytecode instructions")
	}
	if compiled.Bytecode().ResultType != ValueTypeBool {
		t.Fatalf("unexpected bytecode result type: %s", compiled.Bytecode().ResultType)
	}
	if compiled.Fingerprint() == "" {
		t.Fatal("expected compiled fingerprint")
	}
}

func TestBytecodeProgramFingerprintStable(t *testing.T) {
	program := BytecodeProgram{
		Instructions: []Instruction{
			{Op: OpPushConst, Arg: 0},
			{Op: OpPushConst, Arg: 1},
			{Op: OpBinaryAdd},
		},
		Constants:  []any{1, 2},
		ResultType: ValueTypeLong,
	}

	first := program.Fingerprint()
	second := program.Fingerprint()
	if first == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if first != second {
		t.Fatalf("expected stable fingerprint, got %q and %q", first, second)
	}
}

func TestBuildCompileCacheKeyChangesWithRegistry(t *testing.T) {
	base := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}
	changed := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
		},
	}

	baseKey, err := BuildCompileCacheKey("f_amount > 100", base)
	if err != nil {
		t.Fatalf("BuildCompileCacheKey returned error: %v", err)
	}
	changedKey, err := BuildCompileCacheKey("f_amount > 100", changed)
	if err != nil {
		t.Fatalf("BuildCompileCacheKey returned error: %v", err)
	}
	if baseKey == changedKey {
		t.Fatal("expected different cache keys for different registries")
	}
}

func TestCachedRuleCompilerHitsCache(t *testing.T) {
	cachedExpr := compiledExpr{
		source:     "f_amount > 100",
		bytecode:   BytecodeProgram{Instructions: []Instruction{{Op: OpPushConst, Arg: 0}}, Constants: []any{true}, ResultType: ValueTypeBool},
		resultType: ValueTypeBool,
	}
	inner := &countingRuleCompiler{result: cachedExpr}
	compiler := NewCachedRuleCompiler(inner, NewInMemoryProgramCache())
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}

	first, err := compiler.Compile("f_amount > 100", registry)
	if err != nil {
		t.Fatalf("first Compile returned error: %v", err)
	}
	second, err := compiler.Compile("f_amount > 100", registry)
	if err != nil {
		t.Fatalf("second Compile returned error: %v", err)
	}
	if inner.compileCalls != 1 {
		t.Fatalf("expected one inner compile, got %d", inner.compileCalls)
	}
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("expected cached compiled expr fingerprint to match, got %q and %q", first.Fingerprint(), second.Fingerprint())
	}
}

func TestCompileFactorAccessor(t *testing.T) {
	factor := &FactorDefinition{
		FactorCode: "f_user_info",
		OutputSchema: Schema{
			"profile": {
				Type: ValueTypeObject,
				Fields: map[string]*FieldSchema{
					"account_status": {Type: ValueTypeString, Required: false},
				},
			},
		},
	}

	accessor, err := CompileFactorAccessor(FactorRef{
		FactorCode:   "f_user_info",
		PathSegments: []string{"profile", "account_status"},
	}, factor)
	if err != nil {
		t.Fatalf("CompileFactorAccessor returned error: %v", err)
	}

	value, ok := accessor.Get(EvalContext{
		"f_user_info": map[string]any{
			"profile": map[string]any{
				"account_status": "active",
			},
		},
	})
	if !ok || value != "active" {
		t.Fatalf("unexpected accessor result: %v, %v", value, ok)
	}
}

func TestCompileFactorAccessorSingleValueFactor(t *testing.T) {
	factor := &FactorDefinition{
		FactorCode: "f_amount",
		OutputSchema: Schema{
			"value": {Type: ValueTypeLong, Required: true},
		},
	}

	accessor, err := CompileFactorAccessor(FactorRef{FactorCode: "f_amount"}, factor)
	if err != nil {
		t.Fatalf("CompileFactorAccessor returned error: %v", err)
	}

	value, ok := accessor.Get(EvalContext{
		"f_amount": map[string]any{
			"value": int64(1000),
		},
	})
	if !ok || value != int64(1000) {
		t.Fatalf("unexpected accessor result: %v, %v", value, ok)
	}
}

func TestCompiledBoundTreeUsesAccessor(t *testing.T) {
	registry := FactorRegistry{
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("f_user_info.register_days < 7", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	root, ok := compiled.Bound().(BoundBinaryExpr)
	if !ok {
		t.Fatalf("expected bound binary root, got %T", compiled.Bound())
	}
	left, ok := root.Left.(BoundFactorRefExpr)
	if !ok {
		t.Fatalf("expected bound factor ref, got %T", root.Left)
	}
	if left.Accessor == nil {
		t.Fatal("expected factor ref accessor to be compiled")
	}
}

func TestCompiledBoundTreeUsesPreboundEvaluators(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("f_amount > 100000 && f_user_info.register_days < 7", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	root, ok := compiled.Bound().(BoundBinaryExpr)
	if !ok {
		t.Fatalf("expected bound binary root, got %T", compiled.Bound())
	}
	if root.Evaluator == nil {
		t.Fatal("expected root binary evaluator to be prebound")
	}

	leftCompare, ok := root.Left.(BoundBinaryExpr)
	if !ok {
		t.Fatalf("expected left compare node, got %T", root.Left)
	}
	if leftCompare.Evaluator == nil {
		t.Fatal("expected nested binary evaluator to be prebound")
	}
}

func TestParseExpressionFunctionCall(t *testing.T) {
	expr, err := ParseExpression("isEmpty(f_user_info.kyc_level) || in(f_level, \"L1\", \"L2\")")
	if err != nil {
		t.Fatalf("ParseExpression returned error: %v", err)
	}
	if _, ok := expr.(BinaryExpr); !ok {
		t.Fatalf("expected root binary expression, got %T", expr)
	}
}

func TestExpressionTypeCheckerFunctionCalls(t *testing.T) {
	registry := FactorRegistry{
		"f_level": {
			FactorCode: "f_level",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"kyc_level": {Type: ValueTypeString, Required: false},
			},
		},
	}
	checker := NewExpressionTypeChecker(registry)
	valueType, err := checker.CheckString("isEmpty(f_user_info.kyc_level) || in(f_level, \"L1\", \"L2\")")
	if err != nil {
		t.Fatalf("CheckString returned error: %v", err)
	}
	if valueType != ValueTypeBool {
		t.Fatalf("unexpected expression type: %s", valueType)
	}
}

func TestCompiledExprEvalFunctionCalls(t *testing.T) {
	registry := FactorRegistry{
		"f_level": {
			FactorCode: "f_level",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"kyc_level": {Type: ValueTypeString, Required: false},
			},
		},
	}
	compiled, err := NewRuleCompiler().Compile("isEmpty(f_user_info.kyc_level) || in(f_level, \"L1\", \"L2\")", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{
		"f_level": "L2",
		"f_user_info": map[string]any{
			"kyc_level": "",
		},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestExpressionTypeCheckerUnknownFunction(t *testing.T) {
	registry := FactorRegistry{
		"f_level": {
			FactorCode: "f_level",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
	}
	_, err := NewExpressionTypeChecker(registry).CheckString("unknownFn(f_level)")
	if err == nil {
		t.Fatal("expected unknown function error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrUnknownFunction {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseExpressionListLiteral(t *testing.T) {
	expr, err := ParseExpression("contains([1, 2, 3], f_score)")
	if err != nil {
		t.Fatalf("ParseExpression returned error: %v", err)
	}
	if _, ok := expr.(FunctionCallExpr); !ok {
		t.Fatalf("expected function call expression, got %T", expr)
	}
}

func TestExpressionTypeCheckerContainsWithList(t *testing.T) {
	registry := FactorRegistry{
		"f_score": {
			FactorCode: "f_score",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
		},
		"f_text": {
			FactorCode: "f_text",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
	}
	checker := NewExpressionTypeChecker(registry)
	valueType, err := checker.CheckString("contains([1, 2, 3], f_score) && contains(f_text, \"abc\")")
	if err != nil {
		t.Fatalf("CheckString returned error: %v", err)
	}
	if valueType != ValueTypeBool {
		t.Fatalf("unexpected expression type: %s", valueType)
	}
}

func TestCompiledExprEvalContainsWithList(t *testing.T) {
	registry := FactorRegistry{
		"f_score": {
			FactorCode: "f_score",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
		},
	}
	compiled, err := NewRuleCompiler().Compile("contains([1, 2, 3], f_score)", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{
		"f_score": 2,
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestCompiledExprEvalContainsWithString(t *testing.T) {
	registry := FactorRegistry{
		"f_text": {
			FactorCode: "f_text",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
	}
	compiled, err := NewRuleCompiler().Compile("contains(f_text, \"abc\")", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{
		"f_text": "xxabcxx",
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestParseExpressionMapLiteral(t *testing.T) {
	expr, err := ParseExpression("get({\"a\": 1, \"b\": 2}, \"a\")")
	if err != nil {
		t.Fatalf("ParseExpression returned error: %v", err)
	}
	if _, ok := expr.(FunctionCallExpr); !ok {
		t.Fatalf("expected function call expression, got %T", expr)
	}
}

func TestExpressionTypeCheckerGetWithMap(t *testing.T) {
	checker := NewExpressionTypeChecker(FactorRegistry{})
	valueType, err := checker.CheckString("get({\"a\": 1, \"b\": 2}, \"a\") < 3")
	if err != nil {
		t.Fatalf("CheckString returned error: %v", err)
	}
	if valueType != ValueTypeBool {
		t.Fatalf("unexpected expression type: %s", valueType)
	}
}

func TestCompiledExprEvalGetWithMap(t *testing.T) {
	compiled, err := NewRuleCompiler().Compile("get({\"a\": 1, \"b\": f_score}, \"b\")", FactorRegistry{
		"f_score": {
			FactorCode: "f_score",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{
		"f_score": 7,
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != 7 {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestExpressionTypeCheckerRejectsGetOnNonMap(t *testing.T) {
	registry := FactorRegistry{
		"f_text": {
			FactorCode: "f_text",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
	}
	_, err := NewExpressionTypeChecker(registry).CheckString("get(f_text, \"a\")")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpressionTypeCheckerGetOnObjectFactor(t *testing.T) {
	registry := FactorRegistry{
		"f_device_info": {
			FactorCode: "f_device_info",
			OutputSchema: Schema{
				"risk": {
					Type: ValueTypeObject,
					Fields: map[string]*FieldSchema{
						"level": {Type: ValueTypeString, Required: true},
						"score": {Type: ValueTypeInt, Required: true},
					},
				},
			},
		},
	}

	checker := NewExpressionTypeChecker(registry)
	valueType, err := checker.CheckString("get(get(f_device_info, \"risk\"), \"level\") == \"high\" && get(get(f_device_info, \"risk\"), \"score\") > 80")
	if err != nil {
		t.Fatalf("CheckString returned error: %v", err)
	}
	if valueType != ValueTypeBool {
		t.Fatalf("unexpected expression type: %s", valueType)
	}
}

func TestCompiledExprEvalGetOnObjectFactor(t *testing.T) {
	registry := FactorRegistry{
		"f_device_info": {
			FactorCode: "f_device_info",
			OutputSchema: Schema{
				"risk": {
					Type: ValueTypeObject,
					Fields: map[string]*FieldSchema{
						"level": {Type: ValueTypeString, Required: true},
						"score": {Type: ValueTypeInt, Required: true},
					},
				},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("get(get(f_device_info, \"risk\"), \"level\") == \"high\" && get(get(f_device_info, \"risk\"), \"score\") > 80", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{
		"f_device_info": map[string]any{
			"risk": map[string]any{
				"level": "high",
				"score": 95,
			},
		},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestExpressionTypeCheckerRejectsGetOnObjectWithDynamicKey(t *testing.T) {
	registry := FactorRegistry{
		"f_device_info": {
			FactorCode: "f_device_info",
			OutputSchema: Schema{
				"risk": {
					Type: ValueTypeObject,
					Fields: map[string]*FieldSchema{
						"level": {Type: ValueTypeString, Required: true},
					},
				},
			},
		},
		"f_key": {
			FactorCode: "f_key",
			OutputSchema: Schema{
				"value": {Type: ValueTypeString, Required: true},
			},
		},
	}

	_, err := NewExpressionTypeChecker(registry).CheckString("get(get(f_device_info, \"risk\"), f_key)")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseExpressionConditionalAndNull(t *testing.T) {
	expr, err := ParseExpression("f_is_vip ? null : \"guest\"")
	if err != nil {
		t.Fatalf("ParseExpression returned error: %v", err)
	}
	if _, ok := expr.(ConditionalExpr); !ok {
		t.Fatalf("expected conditional expression, got %T", expr)
	}
}

func TestExpressionTypeCheckerConditionalWithNull(t *testing.T) {
	registry := FactorRegistry{
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}
	valueType, err := NewExpressionTypeChecker(registry).CheckString("f_is_vip ? null : \"guest\"")
	if err != nil {
		t.Fatalf("CheckString returned error: %v", err)
	}
	if valueType != ValueTypeString {
		t.Fatalf("unexpected expression type: %s", valueType)
	}
}

func TestExpressionTypeCheckerRejectsConditionalNonBoolCondition(t *testing.T) {
	registry := FactorRegistry{
		"f_score": {
			FactorCode: "f_score",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
		},
	}
	_, err := NewExpressionTypeChecker(registry).CheckString("f_score ? 1 : 0")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpressionTypeCheckerRejectsConditionalIncompatibleBranches(t *testing.T) {
	registry := FactorRegistry{
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}
	_, err := NewExpressionTypeChecker(registry).CheckString("f_is_vip ? 1 : \"guest\"")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompiledExprEvalConditionalWithNull(t *testing.T) {
	registry := FactorRegistry{
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}
	compiled, err := NewRuleCompiler().Compile("f_is_vip ? null : \"guest\"", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{"f_is_vip": true})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
	result, err = compiled.Eval(EvalContext{"f_is_vip": false})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != "guest" {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestCompiledExprEvalNestedConditional(t *testing.T) {
	registry := FactorRegistry{
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
		"f_has_coupon": {
			FactorCode: "f_has_coupon",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}
	compiled, err := NewRuleCompiler().Compile("f_is_vip ? \"vip\" : (f_has_coupon ? \"coupon\" : \"guest\")", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	result, err := compiled.Eval(EvalContext{
		"f_is_vip":     false,
		"f_has_coupon": true,
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != "coupon" {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestBytecodeVMEvalMatchesCompiledEval(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
				"kyc_level":     {Type: ValueTypeString, Required: false},
			},
		},
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("f_amount > 100000 && (isEmpty(f_user_info.kyc_level) ? f_is_vip : false)", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	ctx := EvalContext{
		"f_amount": int64(200000),
		"f_user_info": map[string]any{
			"register_days": 3,
			"kyc_level":     "",
		},
		"f_is_vip": true,
	}

	compiledResult, err := compiled.Eval(ctx)
	if err != nil {
		t.Fatalf("Compiled Eval returned error: %v", err)
	}

	vmResult, err := NewBytecodeVM().Eval(compiled.Bytecode(), ctx)
	if err != nil {
		t.Fatalf("VM Eval returned error: %v", err)
	}

	if compiledResult != vmResult {
		t.Fatalf("expected VM result to match compiled eval: compiled=%v vm=%v", compiledResult, vmResult)
	}
}

func TestBytecodeUsesPools(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("contains([1, 2, 1], f_amount) ? \"hit\" : \"miss\"", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	bytecode := compiled.Bytecode()
	if len(bytecode.Constants) == 0 {
		t.Fatal("expected constant pool to be populated")
	}
	if len(bytecode.Accessors) == 0 {
		t.Fatal("expected accessor pool to be populated")
	}
	if len(bytecode.Builtins) == 0 {
		t.Fatal("expected builtin pool to be populated")
	}

	foundConst := false
	foundLoad := false
	foundBuiltin := false
	for _, inst := range bytecode.Instructions {
		switch inst.Op {
		case OpPushConst:
			foundConst = true
			if inst.Arg < 0 || inst.Arg >= len(bytecode.Constants) {
				t.Fatalf("invalid constant pool index: %d", inst.Arg)
			}
		case OpLoadFactor:
			foundLoad = true
			if inst.Arg < 0 || inst.Arg >= len(bytecode.Accessors) {
				t.Fatalf("invalid accessor pool index: %d", inst.Arg)
			}
		case OpCallBuiltin:
			foundBuiltin = true
			if inst.Arg < 0 || inst.Arg >= len(bytecode.Builtins) {
				t.Fatalf("invalid builtin pool index: %d", inst.Arg)
			}
		}
	}
	if !foundConst || !foundLoad || !foundBuiltin {
		t.Fatalf("expected pooled instructions, got const=%v load=%v builtin=%v", foundConst, foundLoad, foundBuiltin)
	}
}

func TestBytecodeDisassemble(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("contains([1, 2], f_amount)", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	disasm := compiled.Bytecode().Disassemble()
	if disasm == "" {
		t.Fatal("expected non-empty disassembly")
	}
	if !strings.Contains(disasm, "PUSH_CONST") {
		t.Fatalf("expected PUSH_CONST in disassembly, got:\n%s", disasm)
	}
	if !strings.Contains(disasm, "LOAD_FACTOR") {
		t.Fatalf("expected LOAD_FACTOR in disassembly, got:\n%s", disasm)
	}
	if !strings.Contains(disasm, "CALL_BUILTIN") {
		t.Fatalf("expected CALL_BUILTIN in disassembly, got:\n%s", disasm)
	}
}

func TestBytecodeVMTrace(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("f_amount > 100", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	trace, result, err := NewBytecodeVM().Trace(compiled.Bytecode(), EvalContext{
		"f_amount": int64(200),
	})
	if err != nil {
		t.Fatalf("Trace returned error: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected trace result: %v", result)
	}
	if len(trace) == 0 {
		t.Fatal("expected non-empty trace")
	}
	if trace[0].Instruction == "" {
		t.Fatal("expected instruction text in trace")
	}
}

func TestBytecodeConstantFolding(t *testing.T) {
	compiled, err := NewRuleCompiler().Compile("true ? (1 + 2) : (3 + 4)", FactorRegistry{})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	bytecode := compiled.Bytecode()
	if len(bytecode.Instructions) != 1 {
		t.Fatalf("expected constant-folded single instruction, got %d\n%s", len(bytecode.Instructions), bytecode.Disassemble())
	}
	if bytecode.Instructions[0].Op != OpPushConst {
		t.Fatalf("expected PUSH_CONST after folding, got %v", bytecode.Instructions[0].Op)
	}

	result, err := NewBytecodeVM().Eval(bytecode, nil)
	if err != nil {
		t.Fatalf("VM Eval returned error: %v", err)
	}
	if result != int64(3) {
		t.Fatalf("unexpected folded result: %#v", result)
	}
}

func TestBytecodePeepholeRemovesNoOpJump(t *testing.T) {
	compiled, err := NewRuleCompiler().Compile("f_is_vip ? true : false", FactorRegistry{
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	instructions := compiled.Bytecode().Instructions
	for i, inst := range instructions {
		if inst.Op == OpJump && inst.Arg == i+1 {
			t.Fatalf("found no-op jump at %d\n%s", i, compiled.Bytecode().Disassemble())
		}
	}
}

func TestBytecodeDivisionByZeroReturnsValidationError(t *testing.T) {
	compiled, err := NewRuleCompiler().Compile("1 / 0", FactorRegistry{})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	_, err = NewBytecodeVM().Eval(compiled.Bytecode(), nil)
	if err == nil {
		t.Fatal("expected division by zero error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrDivisionByZero {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBytecodeCompileRecoversPanics(t *testing.T) {
	_, err := NewBytecodeCompiler().Compile(BoundFunctionCallExpr{
		Name:  "panicFold",
		Type:  ValueTypeInt,
		Field: scalarField(ValueTypeInt),
		Evaluator: func(args []BoundExpr, ctx EvalContext) (any, error) {
			panic("compile boom")
		},
	})
	if err == nil {
		t.Fatal("expected compile panic recovery error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrInternalPanic {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBytecodeEvalRecoversPanics(t *testing.T) {
	_, err := NewBytecodeVM().Eval(BytecodeProgram{
		Instructions: []Instruction{{Op: OpLoadFactor, Arg: 0}},
		Accessors:    []ValueAccessor{panicAccessor{}},
	}, EvalContext{})
	if err == nil {
		t.Fatal("expected eval panic recovery error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrInternalPanic {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBytecodeVMMaxStepsBudget(t *testing.T) {
	_, err := NewBytecodeVMWithConfig(BytecodeVMConfig{
		MaxSteps:      3,
		MaxStackDepth: 16,
	}).Eval(BytecodeProgram{
		Instructions: []Instruction{{Op: OpJump, Arg: 0}},
	}, EvalContext{})
	if err == nil {
		t.Fatal("expected step budget error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrExecutionBudget || v.Field != "steps" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBytecodeVMMaxStackBudget(t *testing.T) {
	_, err := NewBytecodeVMWithConfig(BytecodeVMConfig{
		MaxSteps:      8,
		MaxStackDepth: 1,
	}).Eval(BytecodeProgram{
		Instructions: []Instruction{
			{Op: OpPushConst, Arg: 0},
			{Op: OpPushConst, Arg: 1},
		},
		Constants: []any{1, 2},
	}, EvalContext{})
	if err == nil {
		t.Fatal("expected stack budget error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrExecutionBudget || v.Field != "stack" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBytecodeVMContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewBytecodeVMWithConfig(BytecodeVMConfig{
		Context:       ctx,
		MaxSteps:      8,
		MaxStackDepth: 8,
	}).Eval(BytecodeProgram{
		Instructions: []Instruction{{Op: OpJump, Arg: 0}},
	}, EvalContext{})
	if err == nil {
		t.Fatal("expected execution cancelled error")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrExecutionCancelled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBytecodeVMConditionalAndGet(t *testing.T) {
	registry := FactorRegistry{
		"f_device_info": {
			FactorCode: "f_device_info",
			OutputSchema: Schema{
				"risk": {
					Type: ValueTypeObject,
					Fields: map[string]*FieldSchema{
						"level": {Type: ValueTypeString, Required: true},
					},
				},
			},
		},
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("f_is_vip ? get(get(f_device_info, \"risk\"), \"level\") : null", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	result, err := NewBytecodeVM().Eval(compiled.Bytecode(), EvalContext{
		"f_is_vip": true,
		"f_device_info": map[string]any{
			"risk": map[string]any{
				"level": "high",
			},
		},
	})
	if err != nil {
		t.Fatalf("VM Eval returned error: %v", err)
	}
	if result != "high" {
		t.Fatalf("unexpected vm result: %v", result)
	}
}

func TestRuleCompilerCompileRejectsInvalidExpression(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
	}

	compiler := NewRuleCompiler()
	_, err := compiler.Compile("f_amount && 100", registry)
	if err == nil {
		t.Fatal("expected compile to fail on type mismatch")
	}
	if v, ok := err.(ValidationError); !ok || v.Code != ErrTypeMismatch {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompiledExprEvalBooleanExpression(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
			},
		},
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}

	compiled, err := NewRuleCompiler().Compile("f_amount > 100000 && f_user_info.register_days < 7 && f_is_vip", registry)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	result, err := compiled.Eval(EvalContext{
		"f_amount": int64(200000),
		"f_user_info": map[string]any{
			"register_days": 3,
		},
		"f_is_vip": true,
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected eval result: %v", result)
	}
}

func TestCompiledExprEvalArithmeticAndNullSemantics(t *testing.T) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_fee": {
			FactorCode: "f_fee",
			OutputSchema: Schema{
				"value": {Type: ValueTypeInt, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: false},
			},
		},
	}

	compiler := NewRuleCompiler()

	sumExpr, err := compiler.Compile("f_amount + f_fee", registry)
	if err != nil {
		t.Fatalf("Compile sum expression returned error: %v", err)
	}
	sum, err := sumExpr.Eval(EvalContext{
		"f_amount": int64(100),
		"f_fee":    7,
	})
	if err != nil {
		t.Fatalf("Eval sum returned error: %v", err)
	}
	if sum != int64(107) {
		t.Fatalf("unexpected sum result: %#v", sum)
	}

	compareExpr, err := compiler.Compile("f_user_info.register_days < 7", registry)
	if err != nil {
		t.Fatalf("Compile compare expression returned error: %v", err)
	}
	compare, err := compareExpr.Eval(EvalContext{
		"f_user_info": map[string]any{},
	})
	if err != nil {
		t.Fatalf("Eval compare returned error: %v", err)
	}
	if compare != false {
		t.Fatalf("expected nil comparison to be false, got %#v", compare)
	}

	nilArithmetic, err := sumExpr.Eval(EvalContext{
		"f_amount": int64(100),
	})
	if err != nil {
		t.Fatalf("Eval nil arithmetic returned error: %v", err)
	}
	if nilArithmetic != nil {
		t.Fatalf("expected nil arithmetic result, got %#v", nilArithmetic)
	}
}

func BenchmarkRuleCompilerCompile(b *testing.B) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
				"kyc_level":     {Type: ValueTypeString, Required: false},
			},
		},
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}
	expr := "f_amount > 100000 && f_user_info.register_days < 7 && (isEmpty(f_user_info.kyc_level) ? f_is_vip : false)"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := NewRuleCompiler().Compile(expr, registry); err != nil {
			b.Fatalf("Compile returned error: %v", err)
		}
	}
}

func BenchmarkCachedRuleCompilerCompile(b *testing.B) {
	registry := FactorRegistry{
		"f_amount": {
			FactorCode: "f_amount",
			OutputSchema: Schema{
				"value": {Type: ValueTypeLong, Required: true},
			},
		},
		"f_user_info": {
			FactorCode: "f_user_info",
			OutputSchema: Schema{
				"register_days": {Type: ValueTypeInt, Required: true},
				"kyc_level":     {Type: ValueTypeString, Required: false},
			},
		},
		"f_is_vip": {
			FactorCode: "f_is_vip",
			OutputSchema: Schema{
				"value": {Type: ValueTypeBool, Required: true},
			},
		},
	}
	expr := "f_amount > 100000 && f_user_info.register_days < 7 && (isEmpty(f_user_info.kyc_level) ? f_is_vip : false)"
	compiler := NewCachedRuleCompiler(NewRuleCompiler(), NewInMemoryProgramCache())

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compiler.Compile(expr, registry); err != nil {
			b.Fatalf("Compile returned error: %v", err)
		}
	}
}
