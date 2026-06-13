package factorengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type OpCode byte

const (
	OpPushConst OpCode = iota
	OpLoadFactor
	OpMakeList
	OpMakeMap
	OpUnaryNot
	OpUnaryNeg
	OpBinaryLT
	OpBinaryLE
	OpBinaryGT
	OpBinaryGE
	OpBinaryEQ
	OpBinaryNE
	OpBinaryAdd
	OpBinarySub
	OpBinaryMul
	OpBinaryDiv
	OpJumpIfFalse
	OpJumpIfTrue
	OpJump
	OpCallBuiltin
)

type Instruction struct {
	Op  OpCode
	Arg int
	Aux int
}

type BytecodeProgram struct {
	Instructions []Instruction
	Constants    []any
	Accessors    []ValueAccessor
	Builtins     []string
	ResultType   ValueType
}

type TraceStep struct {
	IP          int
	Instruction string
	StackBefore []any
	StackAfter  []any
}

type BytecodeVMConfig struct {
	Context       context.Context
	MaxSteps      int
	MaxStackDepth int
}

type BytecodeCompiler struct {
	constIndex    map[string]int
	accessorIndex map[string]int
	builtinIndex  map[string]int
	constants     []any
	accessors     []ValueAccessor
	builtins      []string
}

func NewBytecodeCompiler() *BytecodeCompiler {
	return &BytecodeCompiler{
		constIndex:    make(map[string]int),
		accessorIndex: make(map[string]int),
		builtinIndex:  make(map[string]int),
	}
}

func (p BytecodeProgram) Disassemble() string {
	var lines []string
	for ip, inst := range p.Instructions {
		lines = append(lines, fmt.Sprintf("%03d  %s", ip, p.formatInstruction(inst)))
	}
	return strings.Join(lines, "\n")
}

func (p BytecodeProgram) Fingerprint() string {
	sum := sha256.Sum256([]byte(p.fingerprintSource()))
	return hex.EncodeToString(sum[:])
}

func (p BytecodeProgram) fingerprintSource() string {
	var builder strings.Builder
	builder.WriteString("result=")
	builder.WriteString(string(p.ResultType))
	builder.WriteByte('|')

	for _, inst := range p.Instructions {
		builder.WriteString(fmt.Sprintf("%d:%d:%d;", inst.Op, inst.Arg, inst.Aux))
	}
	builder.WriteByte('|')

	for _, constant := range p.Constants {
		builder.WriteString(fmt.Sprintf("%T=%#v;", constant, constant))
	}
	builder.WriteByte('|')

	for _, accessor := range p.Accessors {
		builder.WriteString(fmt.Sprintf("%#v;", accessor))
	}
	builder.WriteByte('|')

	for _, builtin := range p.Builtins {
		builder.WriteString(builtin)
		builder.WriteByte(';')
	}
	return builder.String()
}

const (
	defaultMaxSteps      = 10000
	defaultMaxStackDepth = 1024
)

func (p BytecodeProgram) formatInstruction(inst Instruction) string {
	switch inst.Op {
	case OpPushConst:
		if inst.Arg >= 0 && inst.Arg < len(p.Constants) {
			return fmt.Sprintf("PUSH_CONST   c[%d]=%#v", inst.Arg, p.Constants[inst.Arg])
		}
		return fmt.Sprintf("PUSH_CONST   c[%d]", inst.Arg)
	case OpLoadFactor:
		if inst.Arg >= 0 && inst.Arg < len(p.Accessors) {
			return fmt.Sprintf("LOAD_FACTOR  a[%d]=%#v", inst.Arg, p.Accessors[inst.Arg])
		}
		return fmt.Sprintf("LOAD_FACTOR  a[%d]", inst.Arg)
	case OpMakeList:
		return fmt.Sprintf("MAKE_LIST    n=%d", inst.Arg)
	case OpMakeMap:
		return fmt.Sprintf("MAKE_MAP     n=%d", inst.Arg)
	case OpUnaryNot:
		return "UNARY_NOT"
	case OpUnaryNeg:
		return "UNARY_NEG"
	case OpBinaryLT:
		return "BINARY_LT"
	case OpBinaryLE:
		return "BINARY_LE"
	case OpBinaryGT:
		return "BINARY_GT"
	case OpBinaryGE:
		return "BINARY_GE"
	case OpBinaryEQ:
		return "BINARY_EQ"
	case OpBinaryNE:
		return "BINARY_NE"
	case OpBinaryAdd:
		return "BINARY_ADD"
	case OpBinarySub:
		return "BINARY_SUB"
	case OpBinaryMul:
		return "BINARY_MUL"
	case OpBinaryDiv:
		return "BINARY_DIV"
	case OpJumpIfFalse:
		return fmt.Sprintf("JUMP_IF_FALSE -> %d", inst.Arg)
	case OpJumpIfTrue:
		return fmt.Sprintf("JUMP_IF_TRUE  -> %d", inst.Arg)
	case OpJump:
		return fmt.Sprintf("JUMP          -> %d", inst.Arg)
	case OpCallBuiltin:
		if inst.Arg >= 0 && inst.Arg < len(p.Builtins) {
			return fmt.Sprintf("CALL_BUILTIN  b[%d]=%s argc=%d", inst.Arg, p.Builtins[inst.Arg], inst.Aux)
		}
		return fmt.Sprintf("CALL_BUILTIN  b[%d] argc=%d", inst.Arg, inst.Aux)
	default:
		return fmt.Sprintf("UNKNOWN_OP(%d)", inst.Op)
	}
}

func (c *BytecodeCompiler) Compile(expr BoundExpr) (_ BytecodeProgram, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = panicAsValidationError("bytecode_compile", r)
		}
	}()
	expr, err = c.optimizeBoundExpr(expr)
	if err != nil {
		return BytecodeProgram{}, err
	}
	var instructions []Instruction
	if err := c.emitExpr(expr, &instructions); err != nil {
		return BytecodeProgram{}, err
	}
	instructions = optimizeInstructions(instructions)
	return BytecodeProgram{
		Instructions: instructions,
		Constants:    append([]any(nil), c.constants...),
		Accessors:    append([]ValueAccessor(nil), c.accessors...),
		Builtins:     append([]string(nil), c.builtins...),
		ResultType:   expr.ResultType(),
	}, nil
}

func (c *BytecodeCompiler) optimizeBoundExpr(expr BoundExpr) (BoundExpr, error) {
	switch e := expr.(type) {
	case BoundLiteralExpr:
		return e, nil
	case BoundFactorRefExpr:
		return e, nil
	case BoundListExpr:
		elems := make([]BoundExpr, 0, len(e.Elements))
		allLiteral := true
		for _, elem := range e.Elements {
			opt, err := c.optimizeBoundExpr(elem)
			if err != nil {
				return nil, err
			}
			if _, ok := opt.(BoundLiteralExpr); !ok {
				allLiteral = false
			}
			elems = append(elems, opt)
		}
		e.Elements = elems
		if allLiteral {
			value, err := e.Eval(nil)
			if err != nil {
				return e, nil
			}
			return BoundLiteralExpr{Value: value, Type: e.Type, Field: e.Field}, nil
		}
		return e, nil
	case BoundMapExpr:
		entries := make([]BoundMapEntry, 0, len(e.Entries))
		allLiteral := true
		for _, entry := range e.Entries {
			opt, err := c.optimizeBoundExpr(entry.Value)
			if err != nil {
				return nil, err
			}
			if _, ok := opt.(BoundLiteralExpr); !ok {
				allLiteral = false
			}
			entries = append(entries, BoundMapEntry{Key: entry.Key, Value: opt})
		}
		e.Entries = entries
		if allLiteral {
			value, err := e.Eval(nil)
			if err != nil {
				return e, nil
			}
			return BoundLiteralExpr{Value: value, Type: e.Type, Field: e.Field}, nil
		}
		return e, nil
	case BoundUnaryExpr:
		optExpr, err := c.optimizeBoundExpr(e.Expr)
		if err != nil {
			return nil, err
		}
		e.Expr = optExpr
		if lit, ok := optExpr.(BoundLiteralExpr); ok {
			value, err := e.Evaluator(lit.Value)
			if err == nil {
				return BoundLiteralExpr{Value: value, Type: e.Type, Field: e.Field}, nil
			}
		}
		return e, nil
	case BoundBinaryExpr:
		left, err := c.optimizeBoundExpr(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := c.optimizeBoundExpr(e.Right)
		if err != nil {
			return nil, err
		}
		e.Left = left
		e.Right = right
		if lit, ok := foldBinaryIfPossible(e); ok {
			return lit, nil
		}
		return e, nil
	case BoundConditionalExpr:
		cond, err := c.optimizeBoundExpr(e.Cond)
		if err != nil {
			return nil, err
		}
		thenExpr, err := c.optimizeBoundExpr(e.Then)
		if err != nil {
			return nil, err
		}
		elseExpr, err := c.optimizeBoundExpr(e.Else)
		if err != nil {
			return nil, err
		}
		e.Cond = cond
		e.Then = thenExpr
		e.Else = elseExpr
		if lit, ok := cond.(BoundLiteralExpr); ok {
			if truthyBool(lit.Value) {
				return thenExpr, nil
			}
			return elseExpr, nil
		}
		return e, nil
	case BoundFunctionCallExpr:
		args := make([]BoundExpr, 0, len(e.Args))
		allLiteral := true
		for _, arg := range e.Args {
			opt, err := c.optimizeBoundExpr(arg)
			if err != nil {
				return nil, err
			}
			if _, ok := opt.(BoundLiteralExpr); !ok {
				allLiteral = false
			}
			args = append(args, opt)
		}
		e.Args = args
		if allLiteral {
			value, err := e.Eval(nil)
			if err == nil {
				return BoundLiteralExpr{Value: value, Type: e.Type, Field: e.Field}, nil
			}
		}
		return e, nil
	default:
		return expr, nil
	}
}

func foldBinaryIfPossible(expr BoundBinaryExpr) (BoundLiteralExpr, bool) {
	leftLit, leftOK := expr.Left.(BoundLiteralExpr)
	rightLit, rightOK := expr.Right.(BoundLiteralExpr)

	switch expr.Op {
	case "&&":
		if leftOK {
			if !truthyBool(leftLit.Value) {
				return BoundLiteralExpr{Value: false, Type: expr.Type, Field: expr.Field}, true
			}
			if rightOK {
				return BoundLiteralExpr{Value: truthyBool(rightLit.Value), Type: expr.Type, Field: expr.Field}, true
			}
		}
	case "||":
		if leftOK {
			if truthyBool(leftLit.Value) {
				return BoundLiteralExpr{Value: true, Type: expr.Type, Field: expr.Field}, true
			}
			if rightOK {
				return BoundLiteralExpr{Value: truthyBool(rightLit.Value), Type: expr.Type, Field: expr.Field}, true
			}
		}
	}

	if !leftOK || !rightOK {
		return BoundLiteralExpr{}, false
	}
	value, err := expr.Evaluator(leftLitWrap(leftLit), leftLitWrap(rightLit), nil)
	if err != nil {
		return BoundLiteralExpr{}, false
	}
	return BoundLiteralExpr{Value: value, Type: expr.Type, Field: expr.Field}, true
}

type literalBoundExpr struct {
	BoundLiteralExpr
}

func leftLitWrap(lit BoundLiteralExpr) BoundExpr {
	return literalBoundExpr{BoundLiteralExpr: lit}
}

func optimizeInstructions(instructions []Instruction) []Instruction {
	if len(instructions) == 0 {
		return instructions
	}
	keep := make([]bool, len(instructions))
	for i := range keep {
		keep[i] = true
	}
	for i, inst := range instructions {
		if inst.Op == OpJump && inst.Arg == i+1 {
			keep[i] = false
		}
	}
	indexMap := make([]int, len(instructions)+1)
	next := 0
	for i := range instructions {
		indexMap[i] = next
		if keep[i] {
			next++
		}
	}
	indexMap[len(instructions)] = next

	optimized := make([]Instruction, 0, next)
	for i, inst := range instructions {
		if !keep[i] {
			continue
		}
		switch inst.Op {
		case OpJump, OpJumpIfFalse, OpJumpIfTrue:
			if inst.Arg >= 0 && inst.Arg <= len(instructions) {
				inst.Arg = indexMap[inst.Arg]
			}
		}
		optimized = append(optimized, inst)
	}
	return optimized
}

func (c *BytecodeCompiler) emitExpr(expr BoundExpr, instructions *[]Instruction) error {
	switch e := expr.(type) {
	case BoundLiteralExpr:
		*instructions = append(*instructions, Instruction{Op: OpPushConst, Arg: c.internConst(e.Value)})
	case BoundFactorRefExpr:
		*instructions = append(*instructions, Instruction{Op: OpLoadFactor, Arg: c.internAccessor(e.Accessor)})
	case BoundListExpr:
		for _, elem := range e.Elements {
			if err := c.emitExpr(elem, instructions); err != nil {
				return err
			}
		}
		*instructions = append(*instructions, Instruction{Op: OpMakeList, Arg: len(e.Elements)})
	case BoundMapExpr:
		for _, entry := range e.Entries {
			*instructions = append(*instructions, Instruction{Op: OpPushConst, Arg: c.internConst(entry.Key)})
			if err := c.emitExpr(entry.Value, instructions); err != nil {
				return err
			}
		}
		*instructions = append(*instructions, Instruction{Op: OpMakeMap, Arg: len(e.Entries)})
	case BoundUnaryExpr:
		if err := c.emitExpr(e.Expr, instructions); err != nil {
			return err
		}
		switch e.Op {
		case "!":
			*instructions = append(*instructions, Instruction{Op: OpUnaryNot})
		case "-":
			*instructions = append(*instructions, Instruction{Op: OpUnaryNeg})
		default:
			return ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported unary operator %q", e.Op)}
		}
	case BoundConditionalExpr:
		if err := c.emitExpr(e.Cond, instructions); err != nil {
			return err
		}
		jumpFalseIdx := len(*instructions)
		*instructions = append(*instructions, Instruction{Op: OpJumpIfFalse})
		if err := c.emitExpr(e.Then, instructions); err != nil {
			return err
		}
		jumpEndIdx := len(*instructions)
		*instructions = append(*instructions, Instruction{Op: OpJump})
		(*instructions)[jumpFalseIdx].Arg = len(*instructions)
		if err := c.emitExpr(e.Else, instructions); err != nil {
			return err
		}
		(*instructions)[jumpEndIdx].Arg = len(*instructions)
	case BoundBinaryExpr:
		if e.Op == "&&" || e.Op == "||" {
			return c.emitShortCircuitBinary(e, instructions)
		}
		if err := c.emitExpr(e.Left, instructions); err != nil {
			return err
		}
		if err := c.emitExpr(e.Right, instructions); err != nil {
			return err
		}
		op, err := binaryOpcode(e.Op)
		if err != nil {
			return err
		}
		*instructions = append(*instructions, Instruction{Op: op})
	case BoundFunctionCallExpr:
		for _, arg := range e.Args {
			if err := c.emitExpr(arg, instructions); err != nil {
				return err
			}
		}
		*instructions = append(*instructions, Instruction{Op: OpCallBuiltin, Arg: c.internBuiltin(e.Name), Aux: len(e.Args)})
	default:
		return ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("unsupported bound expression %T", expr)}
	}
	return nil
}

func (c *BytecodeCompiler) emitShortCircuitBinary(expr BoundBinaryExpr, instructions *[]Instruction) error {
	if err := c.emitExpr(expr.Left, instructions); err != nil {
		return err
	}
	if expr.Op == "&&" {
		jumpFalseIdx := len(*instructions)
		*instructions = append(*instructions, Instruction{Op: OpJumpIfFalse})
		if err := c.emitExpr(expr.Right, instructions); err != nil {
			return err
		}
		jumpEndIdx := len(*instructions)
		*instructions = append(*instructions, Instruction{Op: OpJump})
		(*instructions)[jumpFalseIdx].Arg = len(*instructions)
		*instructions = append(*instructions, Instruction{Op: OpPushConst, Arg: c.internConst(false)})
		(*instructions)[jumpEndIdx].Arg = len(*instructions)
		return nil
	}

	jumpTrueIdx := len(*instructions)
	*instructions = append(*instructions, Instruction{Op: OpJumpIfTrue})
	if err := c.emitExpr(expr.Right, instructions); err != nil {
		return err
	}
	jumpEndIdx := len(*instructions)
	*instructions = append(*instructions, Instruction{Op: OpJump})
	(*instructions)[jumpTrueIdx].Arg = len(*instructions)
	*instructions = append(*instructions, Instruction{Op: OpPushConst, Arg: c.internConst(true)})
	(*instructions)[jumpEndIdx].Arg = len(*instructions)
	return nil
}

func (c *BytecodeCompiler) internConst(value any) int {
	key := fmt.Sprintf("%T:%#v", value, value)
	if idx, ok := c.constIndex[key]; ok {
		return idx
	}
	idx := len(c.constants)
	c.constants = append(c.constants, value)
	c.constIndex[key] = idx
	return idx
}

func (c *BytecodeCompiler) internAccessor(accessor ValueAccessor) int {
	key := fmt.Sprintf("%#v", accessor)
	if idx, ok := c.accessorIndex[key]; ok {
		return idx
	}
	idx := len(c.accessors)
	c.accessors = append(c.accessors, accessor)
	c.accessorIndex[key] = idx
	return idx
}

func (c *BytecodeCompiler) internBuiltin(name string) int {
	if idx, ok := c.builtinIndex[name]; ok {
		return idx
	}
	idx := len(c.builtins)
	c.builtins = append(c.builtins, name)
	c.builtinIndex[name] = idx
	return idx
}

func binaryOpcode(op string) (OpCode, error) {
	switch op {
	case "<":
		return OpBinaryLT, nil
	case "<=":
		return OpBinaryLE, nil
	case ">":
		return OpBinaryGT, nil
	case ">=":
		return OpBinaryGE, nil
	case "==":
		return OpBinaryEQ, nil
	case "!=":
		return OpBinaryNE, nil
	case "+":
		return OpBinaryAdd, nil
	case "-":
		return OpBinarySub, nil
	case "*":
		return OpBinaryMul, nil
	case "/":
		return OpBinaryDiv, nil
	default:
		return 0, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported binary operator %q", op)}
	}
}

type BytecodeVM struct {
	config BytecodeVMConfig
}

func NewBytecodeVM() *BytecodeVM {
	return NewBytecodeVMWithConfig(BytecodeVMConfig{})
}

func NewBytecodeVMWithConfig(config BytecodeVMConfig) *BytecodeVM {
	if config.Context == nil {
		config.Context = context.Background()
	}
	if config.MaxSteps <= 0 {
		config.MaxSteps = defaultMaxSteps
	}
	if config.MaxStackDepth <= 0 {
		config.MaxStackDepth = defaultMaxStackDepth
	}
	return &BytecodeVM{config: config}
}

func (vm *BytecodeVM) Eval(program BytecodeProgram, ctx EvalContext) (any, error) {
	result, _, err := vm.eval(program, ctx, false)
	return result, err
}

func (vm *BytecodeVM) Trace(program BytecodeProgram, ctx EvalContext) ([]TraceStep, any, error) {
	result, steps, err := vm.eval(program, ctx, true)
	return steps, result, err
}

func (vm *BytecodeVM) eval(program BytecodeProgram, ctx EvalContext, trace bool) (any, []TraceStep, error) {
	return vm.evalSafe(program, ctx, trace)
}

func (vm *BytecodeVM) evalSafe(program BytecodeProgram, ctx EvalContext, trace bool) (_ any, _ []TraceStep, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = panicAsValidationError("bytecode_eval", r)
		}
	}()
	stack := make([]any, 0, 16)
	var steps []TraceStep
	ip := 0
	executed := 0
	for ip < len(program.Instructions) {
		if err := vm.checkExecutionLimits(executed, len(stack)); err != nil {
			return nil, steps, err
		}
		executed++
		inst := program.Instructions[ip]
		var before []any
		if trace {
			before = append([]any(nil), stack...)
		}
		switch inst.Op {
		case OpPushConst:
			if inst.Arg < 0 || inst.Arg >= len(program.Constants) {
				return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "invalid constant index"}
			}
			stack = append(stack, program.Constants[inst.Arg])
		case OpLoadFactor:
			if inst.Arg < 0 || inst.Arg >= len(program.Accessors) {
				return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "invalid accessor index"}
			}
			value, ok := program.Accessors[inst.Arg].Get(ctx)
			if !ok {
				stack = append(stack, nil)
			} else {
				stack = append(stack, value)
			}
		case OpMakeList:
			if len(stack) < inst.Arg {
				return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "stack underflow in MAKE_LIST"}
			}
			start := len(stack) - inst.Arg
			values := append([]any(nil), stack[start:]...)
			stack = stack[:start]
			stack = append(stack, values)
		case OpMakeMap:
			need := inst.Arg * 2
			if len(stack) < need {
				return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "stack underflow in MAKE_MAP"}
			}
			start := len(stack) - need
			values := make(map[string]any, inst.Arg)
			for i := start; i < len(stack); i += 2 {
				key, ok := stack[i].(string)
				if !ok {
					return nil, steps, ValidationError{Code: ErrTypeMismatch, Message: "map key must be STRING"}
				}
				values[key] = stack[i+1]
			}
			stack = stack[:start]
			stack = append(stack, values)
		case OpUnaryNot, OpUnaryNeg:
			value, err := vm.pop(&stack)
			if err != nil {
				return nil, steps, err
			}
			var result any
			switch inst.Op {
			case OpUnaryNot:
				result, err = evalNot(value)
			case OpUnaryNeg:
				result, err = evalNegate(value)
			}
			if err != nil {
				return nil, steps, err
			}
			stack = append(stack, result)
		case OpBinaryLT, OpBinaryLE, OpBinaryGT, OpBinaryGE, OpBinaryEQ, OpBinaryNE, OpBinaryAdd, OpBinarySub, OpBinaryMul, OpBinaryDiv:
			right, err := vm.pop(&stack)
			if err != nil {
				return nil, steps, err
			}
			left, err := vm.pop(&stack)
			if err != nil {
				return nil, steps, err
			}
			result, err := evalBinaryValues(inst.Op, left, right)
			if err != nil {
				return nil, steps, err
			}
			stack = append(stack, result)
		case OpJumpIfFalse:
			value, err := vm.pop(&stack)
			if err != nil {
				return nil, steps, err
			}
			if !truthyBool(value) {
				if err := vm.validateJumpTarget(inst.Arg, len(program.Instructions)); err != nil {
					return nil, steps, err
				}
				if trace {
					steps = append(steps, TraceStep{
						IP:          ip,
						Instruction: program.formatInstruction(inst),
						StackBefore: before,
						StackAfter:  append([]any(nil), stack...),
					})
				}
				ip = inst.Arg
				continue
			}
		case OpJumpIfTrue:
			value, err := vm.pop(&stack)
			if err != nil {
				return nil, steps, err
			}
			if truthyBool(value) {
				if err := vm.validateJumpTarget(inst.Arg, len(program.Instructions)); err != nil {
					return nil, steps, err
				}
				if trace {
					steps = append(steps, TraceStep{
						IP:          ip,
						Instruction: program.formatInstruction(inst),
						StackBefore: before,
						StackAfter:  append([]any(nil), stack...),
					})
				}
				ip = inst.Arg
				continue
			}
		case OpJump:
			if err := vm.validateJumpTarget(inst.Arg, len(program.Instructions)); err != nil {
				return nil, steps, err
			}
			if trace {
				steps = append(steps, TraceStep{
					IP:          ip,
					Instruction: program.formatInstruction(inst),
					StackBefore: before,
					StackAfter:  append([]any(nil), stack...),
				})
			}
			ip = inst.Arg
			continue
		case OpCallBuiltin:
			if inst.Arg < 0 || inst.Arg >= len(program.Builtins) {
				return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "invalid builtin index"}
			}
			if len(stack) < inst.Aux {
				return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "stack underflow in CALL_BUILTIN"}
			}
			start := len(stack) - inst.Aux
			args := append([]any(nil), stack[start:]...)
			stack = stack[:start]
			result, err := evalBuiltinValues(program.Builtins[inst.Arg], args)
			if err != nil {
				return nil, steps, err
			}
			stack = append(stack, result)
		default:
			return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("unsupported opcode %d", inst.Op)}
		}
		if len(stack) > vm.config.MaxStackDepth {
			return nil, steps, ValidationError{
				Code:    ErrExecutionBudget,
				Field:   "stack",
				Message: fmt.Sprintf("max stack depth exceeded: %d > %d", len(stack), vm.config.MaxStackDepth),
			}
		}
		if trace {
			steps = append(steps, TraceStep{
				IP:          ip,
				Instruction: program.formatInstruction(inst),
				StackBefore: before,
				StackAfter:  append([]any(nil), stack...),
			})
		}
		ip++
	}
	if len(stack) != 1 {
		return nil, steps, ValidationError{Code: ErrExpressionInvalid, Message: "vm finished with invalid stack state"}
	}
	return stack[0], steps, nil
}

func (vm *BytecodeVM) checkExecutionLimits(executed int, stackDepth int) error {
	if err := vm.config.Context.Err(); err != nil {
		return ValidationError{Code: ErrExecutionCancelled, Message: err.Error()}
	}
	if executed >= vm.config.MaxSteps {
		return ValidationError{
			Code:    ErrExecutionBudget,
			Field:   "steps",
			Message: fmt.Sprintf("max steps exceeded: %d", vm.config.MaxSteps),
		}
	}
	if stackDepth > vm.config.MaxStackDepth {
		return ValidationError{
			Code:    ErrExecutionBudget,
			Field:   "stack",
			Message: fmt.Sprintf("max stack depth exceeded: %d > %d", stackDepth, vm.config.MaxStackDepth),
		}
	}
	return nil
}

func (vm *BytecodeVM) validateJumpTarget(target int, instructionCount int) error {
	if target < 0 || target > instructionCount {
		return ValidationError{
			Code:    ErrExpressionInvalid,
			Field:   "jump",
			Message: fmt.Sprintf("invalid jump target %d", target),
		}
	}
	return nil
}

func (vm *BytecodeVM) pop(stack *[]any) (any, error) {
	if len(*stack) == 0 {
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "stack underflow"}
	}
	last := len(*stack) - 1
	value := (*stack)[last]
	*stack = (*stack)[:last]
	return value, nil
}

func evalBinaryValues(op OpCode, left any, right any) (any, error) {
	switch op {
	case OpBinaryLT:
		return orderedCompare(left, right, "<")
	case OpBinaryLE:
		return orderedCompare(left, right, "<=")
	case OpBinaryGT:
		return orderedCompare(left, right, ">")
	case OpBinaryGE:
		return orderedCompare(left, right, ">=")
	case OpBinaryEQ:
		return equalityCompare(left, right), nil
	case OpBinaryNE:
		return !equalityCompare(left, right), nil
	case OpBinaryAdd:
		return arithmetic(left, right, "+")
	case OpBinarySub:
		return arithmetic(left, right, "-")
	case OpBinaryMul:
		return arithmetic(left, right, "*")
	case OpBinaryDiv:
		return arithmetic(left, right, "/")
	default:
		return nil, ValidationError{Code: ErrUnsupportedOperator, Message: fmt.Sprintf("unsupported binary opcode %q", op)}
	}
}

func evalBuiltinValues(name string, args []any) (any, error) {
	switch name {
	case "isEmpty":
		return builtinIsEmpty(args)
	case "exists":
		return builtinExists(args)
	case "in":
		return builtinIn(args)
	case "contains":
		return builtinContains(args)
	case "get":
		return builtinGet(args)
	default:
		return nil, ValidationError{Code: ErrUnknownFunction, Field: name, Message: "unknown function"}
	}
}

func panicAsValidationError(field string, recovered any) ValidationError {
	return ValidationError{
		Code:    ErrInternalPanic,
		Field:   field,
		Message: fmt.Sprintf("recovered panic: %v", recovered),
	}
}
