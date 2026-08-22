package semantic

import (
	"fmt"
	"math"
	"math/big"
	"slices"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func Analyze(globalFuncs []*ir.Node) ([]*ir.Node, error) {
	globalScope := ir.NewTable()
	a := &analyzer{}

	// register every global function's signature first
	for _, f := range globalFuncs {
		if err := a.registerGlobalFunction(globalScope, f); err != nil {
			return nil, err
		}
	}

	for _, f := range globalFuncs {
		if err := a.analyzeFunctionBody(globalScope, f); err != nil {
			return nil, err
		}
	}

	return slices.Concat(globalFuncs, a.lambdas), nil
}

type analyzer struct {
	lambdas []*ir.Node
}

func (a *analyzer) analyzeStmt(scope *ir.Table, n *ir.Node) error {
	switch n.Op {
	case ir.OpReturn:
		return a.analyzeReturn(scope, n)
	case ir.OpDeclaration:
		return a.analyzeDeclaration(scope, n)
	case ir.OpAssignment:
		return a.analyzeAssignment(scope, n)
	case ir.OpPlusEq, ir.OpMinusEq, ir.OpDivEq, ir.OpTimesEq:
		// assignment operators have same structure and use same typing rules as regular assignment
		return a.analyzeAssignment(scope, n)
	case ir.OpCall:
		return a.inferCall(scope, n)
	default:
		return diagnostic.NewError(n.Pos, "unknown statement operation: %d", n.Op)
	}
}

func (a *analyzer) analyzeAssignment(scope *ir.Table, n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "variable assignment missing target or expression")
	}
	target := n.List[0]
	e := n.List[1]

	// the parser accepts any expression as a target; reject non-lvalues before resolving it
	if !target.IsLValue() {
		return diagnostic.NewError(target.Pos, "invalid assignment target: expression is not assignable")
	}

	// an lvalue carries its own type, so the target is always synthesized
	if err := a.inferExpr(scope, target); err != nil {
		return err
	}

	// the target's type is what the context expects of the right-hand side
	if err := a.checkExpr(scope, e, target.Type); err != nil {
		return err
	}

	// make sure the expression and target type match
	if !types.Equal(target.Type, e.Type) {
		return diagnostic.NewError(n.Pos, "variable assignment with mismatched types: want %v, got %v", target.Type, e.Type)
	}

	return nil
}

func (a *analyzer) analyzeDeclaration(scope *ir.Table, n *ir.Node) error {
	if len(n.List) != 3 {
		return diagnostic.NewError(n.Pos, "variable declaration missing components")
	}
	nameNode := n.List[0]
	typeNode := n.List[1]
	e := n.List[2]

	var want *types.Type
	if typeNode != nil {
		want = typeNode.Type
	}

	// an annotation puts us in checking mode; without one there is nothing to check against
	if want != nil {
		if err := a.checkExpr(scope, e, want); err != nil {
			return err
		}
	} else if err := a.inferExpr(scope, e); err != nil {
		return err
	}

	// the variable's type lands in the symbol table, so it has to be concrete by now
	if err := a.materialize(scope, e); err != nil {
		return err
	}

	// wanted type must equal got type
	if want != nil && !types.Equal(want, e.Type) {
		return diagnostic.NewError(n.Pos, "variable declaration with mismatched types: want %v, got %v", want, e.Type)
	}

	// register self in scope; will get nil if variable already exists in scope
	sym := scope.Register(nameNode.Ident(), e.Type, ir.SymLocal)
	if sym == nil {
		return diagnostic.NewError(nameNode.Pos, "variable re-declared: %v", nameNode.Ident())
	}
	sym.Def = n.Encl() // function context this variable was defined in
	n.Sym = sym

	return nil
}

// inferExpr synthesizes n's type from the expression alone, with no expectation from context.
func (a *analyzer) inferExpr(scope *ir.Table, n *ir.Node) error {
	switch n.Op {
	case ir.OpInt:
		n.Type = types.UntypedInt()
		return nil
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return a.inferBop(scope, n)
	case ir.OpIdent:
		return a.inferIdent(scope, n)
	case ir.OpNegate:
		return a.inferNegate(scope, n)
	case ir.OpCall:
		return a.inferCall(scope, n)
	case ir.OpRef:
		return a.inferRef(scope, n)
	case ir.OpDeref:
		return a.inferDeref(scope, n)
	case ir.OpFunction:
		return a.inferLambda(scope, n)
	case ir.OpTuple:
		return a.inferTuple(scope, n)
	case ir.OpUnit:
		n.Type = types.Unit()
		return nil
	case ir.OpDot:
		return a.inferDot(scope, n)
	default:
		return diagnostic.NewError(n.Pos, "unknown expression operation: %d", n.Op)
	}
}

func (a *analyzer) checkExpr(scope *ir.Table, n *ir.Node, want *types.Type) error {
	switch n.Op {
	case ir.OpInt:
		return a.checkInt(n, want)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return a.checkBop(scope, n, want)
	case ir.OpNegate:
		return a.checkNegate(scope, n, want)
	case ir.OpTuple:
		return a.checkTuple(scope, n, want)
	default:
		// nothing to push down, so synthesize and let the caller compare
		return a.inferExpr(scope, n)
	}
}

// materialize settles any untyped part of n's type on its default
func (a *analyzer) materialize(scope *ir.Table, n *ir.Node) error {
	defaultType := n.Type.ToDefault()
	if types.Equal(defaultType, n.Type) {
		return nil
	}

	if err := a.checkExpr(scope, n, defaultType); err != nil {
		return err
	}
	if !types.Equal(n.Type, defaultType) {
		return diagnostic.NewError(n.Pos, "unable to resolve incomplete type: want %v, got %v", defaultType, n.Type)
	}

	return nil
}

func (a *analyzer) inferDot(scope *ir.Table, n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "dot without two elements")
	}

	left := n.List[0]
	if err := a.inferExpr(scope, left); err != nil {
		return err
	}
	if !left.Type.IsTuple() {
		// TODO: Way in the future expand this out to idents for record access
		return diagnostic.NewError(n.Pos, "dot on non-tuple type: %v", left.Type)
	}

	right := n.List[1]
	if right.Op != ir.OpInt {
		return diagnostic.NewError(right.Pos, "tuples must be accessed with integer literals")
	}
	if err := a.inferExpr(scope, right); err != nil {
		return err
	}

	// synthesized rather than checked so that an out-of-range field reports as such rather than as an overflow
	rightVal := right.Val.(*big.Int)
	if !rightVal.IsInt64() || rightVal.Sign() < 0 || rightVal.Int64() >= int64(len(left.Type.Params())) {
		return diagnostic.NewError(right.Pos, "field %v out-of-range for tuple %v", rightVal, left.Type)
	}

	right.Type = types.Int()

	n.Type = left.Type.Params()[rightVal.Int64()]
	return nil
}

// inferTuple types a tuple literal with nothing expected of it.
func (a *analyzer) inferTuple(scope *ir.Table, n *ir.Node) error {
	typeList := make([]*types.Type, len(n.List))
	for i, e := range n.List {
		if err := a.inferExpr(scope, e); err != nil {
			return err
		}
		if err := a.materialize(scope, e); err != nil {
			return err
		}

		typeList[i] = e.Type
	}

	n.Type = types.Tuple(typeList)

	return nil
}

// checkTuple pushes each component of want into the element sitting at that position.
func (a *analyzer) checkTuple(scope *ir.Table, n *ir.Node, want *types.Type) error {
	if !want.IsTuple() || len(want.Params()) != len(n.List) {
		return a.inferTuple(scope, n)
	}

	typeList := make([]*types.Type, len(n.List))
	for i, e := range n.List {
		if err := a.checkExpr(scope, e, want.Params()[i]); err != nil {
			return err
		}
		if err := a.materialize(scope, e); err != nil {
			return err
		}

		typeList[i] = e.Type
	}

	n.Type = types.Tuple(typeList)

	return nil
}

func (a *analyzer) inferLambda(scope *ir.Table, n *ir.Node) error {
	if n.Signature.Name.Ident() != "" {
		return diagnostic.NewError(n.Pos, "lambda functions must not be named")
	}

	encl := n.Encl()
	if encl == nil {
		return diagnostic.NewError(n.Pos, "lambda without enclosing function")
	}

	n.Signature.Label = fmt.Sprintf("%s.func%d", encl.Signature.Label, encl.NextClosureCount())

	sigType, err := a.signatureType(n)
	if err != nil {
		return err
	}
	n.Type = sigType

	a.lambdas = append(a.lambdas, n)

	return a.analyzeFunctionBody(scope, n)
}

func (a *analyzer) inferRef(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "ref without argument")
	}

	sub := n.List[0]

	// can only take the address of an addressable expression
	if !sub.IsLValue() {
		return diagnostic.NewError(sub.Pos, "cannot take reference of non-addressable expression")
	}

	if err := a.inferExpr(scope, sub); err != nil {
		return err
	}

	// n's type is a pointer of sub's type
	n.Type = types.Pointer(sub.Type)

	return nil
}

func (a *analyzer) inferDeref(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "deref without argument")
	}

	sub := n.List[0]
	if err := a.inferExpr(scope, sub); err != nil {
		return err
	}

	// make sure sub's type is a pointer
	if !sub.Type.IsPointer() {
		return diagnostic.NewError(n.Pos, "dereference of a non-pointer type")
	}

	n.Type = sub.Type.Result()

	return nil
}

func (a *analyzer) inferCall(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "call without a callee")
	}

	// analyze the expression being called
	callee := n.List[0]
	if err := a.inferExpr(scope, callee); err != nil {
		return err
	}

	// make sure the callee is a function
	if !callee.Type.IsFunction() {
		return diagnostic.NewError(n.Pos, "function call on non-function")
	}

	args := n.List[1:]
	params := callee.Type.Params()

	// should have same number of params and args
	if len(params) != len(args) {
		return diagnostic.NewError(n.Pos, "mismatched number of arguments: wanted %d, got %d", len(params), len(args))
	}

	// analyze each argument and make sure its type lines up with that of the matching parameter
	for i := range args {
		arg := args[i]
		param := params[i]

		// the parameter is what this position expects of the argument
		if err := a.checkExpr(scope, arg, param); err != nil {
			return err
		}

		if !types.Equal(param, arg.Type) {
			return diagnostic.NewError(arg.Pos, "type mismatch for call argument: wanted %v, got %v", param, arg.Type)
		}
	}

	// now that we know all is good mark type of call expression as function result
	n.Type = callee.Type.Result()

	return nil
}

func (a *analyzer) inferNegate(scope *ir.Table, n *ir.Node) error {
	e, err := negateOperand(n)
	if err != nil {
		return err
	}

	if err := a.inferExpr(scope, e); err != nil {
		return err
	}

	// steal type from sub-expression
	n.Type = e.Type

	return nil
}

func (a *analyzer) checkNegate(scope *ir.Table, n *ir.Node, want *types.Type) error {
	e, err := negateOperand(n)
	if err != nil {
		return err
	}

	if err := a.checkExpr(scope, e, want); err != nil {
		return err
	}

	n.Type = e.Type

	return nil
}

func negateOperand(n *ir.Node) (*ir.Node, error) {
	if len(n.List) != 1 {
		return nil, diagnostic.NewError(n.Pos, "negation without an argument")
	}

	return n.List[0], nil
}

func (a *analyzer) inferIdent(scope *ir.Table, n *ir.Node) error {
	// need an existing symbol for this ident
	existingSym := scope.Sym(n.Ident())
	if existingSym == nil {
		return diagnostic.NewError(n.Pos, "variable used before declaration: %v", n.Ident())
	}

	n.Type = existingSym.Type
	n.Sym = existingSym

	// make sure this usage's direct enclosing function captures the sym
	n.Encl().Capture(existingSym)

	return nil
}

func (a *analyzer) inferBop(scope *ir.Table, n *ir.Node) error {
	left, right, err := bopOperands(n)
	if err != nil {
		return err
	}

	if err := a.inferExpr(scope, left); err != nil {
		return err
	}
	if err := a.inferExpr(scope, right); err != nil {
		return err
	}

	return a.settleBop(scope, n, left, right)
}

// checkBop hands the expectation to both operands: these operators are homogeneous, so whatever
// the context wants of the result it wants of each side too.
func (a *analyzer) checkBop(scope *ir.Table, n *ir.Node, want *types.Type) error {
	left, right, err := bopOperands(n)
	if err != nil {
		return err
	}

	if err := a.checkExpr(scope, left, want); err != nil {
		return err
	}
	if err := a.checkExpr(scope, right, want); err != nil {
		return err
	}

	return a.settleBop(scope, n, left, right)
}

// settleBop resolves an operand still left untyped against a concrete sibling, then validates the
// pair and types the operator. Shared by both modes: an expectation that failed to reach an operand
// (or that never existed) leaves the same situation behind either way.
func (a *analyzer) settleBop(scope *ir.Table, n *ir.Node, left *ir.Node, right *ir.Node) error {
	switch {
	case left.Type.IsUntypedNumeric() && right.Type.IsConcreteNumeric():
		if err := a.checkExpr(scope, left, right.Type); err != nil {
			return err
		}
	case left.Type.IsConcreteNumeric() && right.Type.IsUntypedNumeric():
		if err := a.checkExpr(scope, right, left.Type); err != nil {
			return err
		}
	}

	// types must be equal
	if !types.Equal(left.Type, right.Type) {
		return diagnostic.NewError(n.Pos, "binary operation with mismatched types: %v and %v", left.Type, right.Type)
	}

	// restrict what can be added to numeric types
	if !left.Type.IsConcreteNumeric() {
		return diagnostic.NewError(n.Pos, "binary operation unavailable for type: %v", left.Type)
	}

	// finally, assign bop node to the agreed-upon type
	n.Type = left.Type

	return nil
}

func bopOperands(n *ir.Node) (*ir.Node, *ir.Node, error) {
	if len(n.List) != 2 {
		return nil, nil, diagnostic.NewError(n.Pos, "binary operator without two operands")
	}

	return n.List[0], n.List[1], nil
}

// terminates reports whether control cannot flow past n.
func terminates(n *ir.Node) bool {
	if n == nil {
		return false
	}

	return n.Op == ir.OpReturn
}

func (a *analyzer) registerGlobalFunction(scope *ir.Table, f *ir.Node) error {
	if f.Signature.Name.Ident() == "" {
		return diagnostic.NewError(f.Pos, "global functions must be named")
	}

	f.Signature.Label = f.Signature.Name.Ident()

	// resolve parameter types and set own type
	sigType, err := a.signatureType(f)
	if err != nil {
		return err
	}

	// TODO: widen this to any integer type once more than one exists.
	if f.Signature.Name.Ident() == "main" && !types.Equal(sigType.Result(), types.Int()) {
		return diagnostic.NewError(f.Pos, "main must return int, got %v", sigType.Result())
	}

	f.Type = sigType

	// register self onto scope
	name := f.Signature.Name
	sym := scope.Register(name.Ident(), f.Type, ir.SymFunc)
	if sym == nil {
		return diagnostic.NewError(name.Pos, "symbol '%s' already declared", name.Ident())
	}
	f.Sym = sym

	return nil
}

func (a *analyzer) analyzeFunctionBody(scope *ir.Table, f *ir.Node) error {
	// need a child scope for function body
	funScope := scope.NewChild()

	// register parameters into the function scope so the body can reference them
	for _, p := range f.Signature.Params {
		pName := p.List[0]
		sym := funScope.Register(pName.Ident(), p.Type, ir.SymParam)
		if sym == nil {
			return diagnostic.NewError(pName.Pos, "parameter '%s' already declared", pName.Ident())
		}
		sym.Def = f // parameter defined in f
		p.Sym = sym
	}

	// analyze types of body statements
	for _, s := range f.List {
		if err := a.analyzeStmt(funScope, s); err != nil {
			return err
		}
	}

	// TODO: point this at the body's closing brace instead of the function's start once nodes carry an end position
	if !(len(f.List) > 0 && terminates(f.List[len(f.List)-1])) && !f.Type.Result().IsUnit() {
		return diagnostic.NewError(f.Pos, "missing return at end of function")
	}

	return nil
}

// signatureType resolves a function node's parameter and result types into the function's own type.
func (a *analyzer) signatureType(f *ir.Node) (*types.Type, error) {
	var paramTypes []*types.Type
	for _, p := range f.Signature.Params {
		if err := a.analyzeParam(p); err != nil {
			return nil, err
		}
		paramTypes = append(paramTypes, p.Type)
	}

	// an omitted result type means the function returns unit
	resultType := types.Unit()
	if f.Signature.Result != nil {
		resultType = f.Signature.Result.Type
	}

	return types.Function(paramTypes, resultType), nil
}

func (a *analyzer) analyzeParam(p *ir.Node) error {
	if len(p.List) != 2 {
		return diagnostic.NewError(p.Pos, "parameter missing type")
	}

	// pull the resolved type from the type node (List[1], after the name) up into the param node
	p.Type = p.List[1].Type

	return nil
}

func (a *analyzer) analyzeReturn(scope *ir.Table, r *ir.Node) error {
	// grab first function we can find in the AST
	currentFunc := r.Predecessor(ir.OpFunction)

	// we expect a return to appear in a function
	if currentFunc == nil {
		return diagnostic.NewError(r.Pos, "return statement appears outside of a function definition")
	}
	expectedOut := currentFunc.Type.Result()

	// determine type of sub-expression
	if len(r.List) > 0 {
		e := r.List[0]

		// the signature's result type is what a return expression is checked against
		if err := a.checkExpr(scope, e, expectedOut); err != nil {
			return err
		}
		if !types.Equal(e.Type, expectedOut) {
			return diagnostic.NewError(e.Pos, "return value type does not match type of function signature. expected %v, got %v", expectedOut, e.Type)
		}
	} else {
		// returning nothing; expect function to return unit type
		if !expectedOut.IsUnit() {
			return diagnostic.NewError(r.Pos, "empty return for function that returns concrete value")
		}
	}

	return nil
}

// checkInt types an integer literal against want, rejecting a value that will not fit
func (a *analyzer) checkInt(i *ir.Node, want *types.Type) error {
	i.Type = types.UntypedInt()

	intVal := i.Val.(*big.Int)

	if types.Equal(want, types.Int()) {
		max32 := big.NewInt(math.MaxInt32)
		min32 := big.NewInt(math.MinInt32)
		if intVal.Cmp(max32) > 0 || intVal.Cmp(min32) < 0 {
			return diagnostic.NewError(i.Pos, "overflow: integer value %v too large for type %v", intVal, types.Int())
		}
		i.Type = types.Int()
	}

	return nil
}
