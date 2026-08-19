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
		return a.analyzeCall(scope, n)
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

	// resolve the target as an expression
	if err := a.analyzeExpr(scope, target, nil); err != nil {
		return err
	}

	// analyze the expression with hint of the target's type
	if err := a.analyzeExpr(scope, e, target.Type); err != nil {
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

	var hint *types.Type
	if typeNode != nil {
		hint = typeNode.Type
	}

	if err := a.analyzeExpr(scope, e, hint); err != nil {
		return err
	}

	// we need a concrete type at this point to resolve any unknowns. must re-analyze with hint if type changes.
	defaultType := e.Type.ToDefault()
	if !types.Equal(defaultType, e.Type) {
		if err := a.analyzeExpr(scope, e, defaultType); err != nil {
			return err
		}
		if !types.Equal(e.Type, defaultType) {
			return diagnostic.NewError(n.Pos, "unable to resolve incomplete type: want %v, got %v", defaultType, e.Type)
		}
	}

	// wanted type must equal got type
	if hint != nil && !types.Equal(hint, e.Type) {
		return diagnostic.NewError(n.Pos, "variable declaration with mismatched types: want %v, got %v", hint, e.Type)
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

func (a *analyzer) analyzeExpr(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	switch n.Op {
	case ir.OpInt:
		return a.analyzeInt(n, hint)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return a.analyzeBop(scope, n, hint)
	case ir.OpIdent:
		return a.analyzeIdent(scope, n)
	case ir.OpNegate:
		return a.analyzeNegate(scope, n, hint)
	case ir.OpCall:
		return a.analyzeCall(scope, n)
	case ir.OpRef:
		return a.analyzeRef(scope, n)
	case ir.OpDeref:
		return a.analyzeDeref(scope, n)
	case ir.OpFunction:
		return a.analyzeLambda(scope, n)
	case ir.OpTuple:
		return a.analyzeTuple(scope, n, hint)
	case ir.OpUnit:
		n.Type = types.Unit()
		return nil
	case ir.OpDot:
		return a.analyzeDot(scope, n)
	default:
		return diagnostic.NewError(n.Pos, "unknown expression operation: %d", n.Op)
	}
}

func (a *analyzer) analyzeDot(scope *ir.Table, n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "dot without two elements")
	}

	left := n.List[0]
	if err := a.analyzeExpr(scope, left, nil); err != nil {
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
	if err := a.analyzeExpr(scope, right, nil); err != nil {
		return err
	}

	// analyzed without a hint so that an out-of-range field reports as such rather than as an overflow
	rightVal := right.Val.(*big.Int)
	if !rightVal.IsInt64() || rightVal.Sign() < 0 || rightVal.Int64() >= int64(len(left.Type.Params())) {
		return diagnostic.NewError(right.Pos, "field %v out-of-range for tuple %v", rightVal, left.Type)
	}

	right.Type = types.Int()

	n.Type = left.Type.Params()[rightVal.Int64()]
	return nil
}

func (a *analyzer) analyzeTuple(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	// hint is only valid if it's the same shape as the tuple
	useHint := hint.IsTuple() && len(hint.Params()) == len(n.List)

	typeList := make([]*types.Type, len(n.List))
	for i, e := range n.List {
		var elemHint *types.Type
		if useHint {
			elemHint = hint.Params()[i]
		}

		if err := a.analyzeExpr(scope, e, elemHint); err != nil {
			return err
		}

		// any element left untyped gets defaulted
		defaultType := e.Type.ToDefault()
		if !types.Equal(defaultType, e.Type) {
			if err := a.analyzeExpr(scope, e, defaultType); err != nil {
				return err
			}
			if !types.Equal(e.Type, defaultType) {
				return diagnostic.NewError(e.Pos, "unable to resolve incomplete type: want %v, got %v", defaultType, e.Type)
			}
		}

		typeList[i] = e.Type
	}

	n.Type = types.Tuple(typeList)

	return nil
}

func (a *analyzer) analyzeLambda(scope *ir.Table, n *ir.Node) error {
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

func (a *analyzer) analyzeRef(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "ref without argument")
	}

	sub := n.List[0]

	// can only take the address of an addressable expression
	if !sub.IsLValue() {
		return diagnostic.NewError(sub.Pos, "cannot take reference of non-addressable expression")
	}

	if err := a.analyzeExpr(scope, sub, nil); err != nil {
		return err
	}

	// n's type is a pointer of sub's type
	n.Type = types.Pointer(sub.Type)

	return nil
}

func (a *analyzer) analyzeDeref(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "deref without argument")
	}

	sub := n.List[0]
	if err := a.analyzeExpr(scope, sub, nil); err != nil {
		return err
	}

	// make sure sub's type is a pointer
	if !sub.Type.IsPointer() {
		return diagnostic.NewError(n.Pos, "dereference of a non-pointer type")
	}

	n.Type = sub.Type.Result()

	return nil
}

func (a *analyzer) analyzeCall(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "call without a callee")
	}

	// analyze the expression being called
	callee := n.List[0]
	if err := a.analyzeExpr(scope, callee, nil); err != nil {
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

		if err := a.analyzeExpr(scope, arg, param); err != nil {
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

func (a *analyzer) analyzeNegate(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	if len(n.List) != 1 {
		return diagnostic.NewError(n.Pos, "negation without an argument")
	}

	// analyze sub-expression with hint
	e := n.List[0]
	if err := a.analyzeExpr(scope, e, hint); err != nil {
		return err
	}

	// steal type from sub-expression
	n.Type = e.Type

	return nil
}

func (a *analyzer) analyzeIdent(scope *ir.Table, n *ir.Node) error {
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

func (a *analyzer) analyzeBop(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	// extract left and right operands
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "binary operator without two operands")
	}
	left := n.List[0]
	right := n.List[1]

	// figure out types of left and right operands given context
	if err := a.analyzeExpr(scope, left, hint); err != nil {
		return err
	}
	if err := a.analyzeExpr(scope, right, hint); err != nil {
		return err
	}

	// attempt to resolve flexible types
	switch {
	case left.Type.IsUntypedNumeric() && right.Type.IsConcreteNumeric():
		if err := a.analyzeExpr(scope, left, right.Type); err != nil {
			return err
		}
	case left.Type.IsConcreteNumeric() && right.Type.IsUntypedNumeric():
		if err := a.analyzeExpr(scope, right, left.Type); err != nil {
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
		if err := a.analyzeExpr(scope, e, expectedOut); err != nil {
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

func (a *analyzer) analyzeInt(i *ir.Node, hint *types.Type) error {
	i.Type = types.UntypedInt()

	intVal := i.Val.(*big.Int)

	if types.Equal(hint, types.Int()) {
		max32 := big.NewInt(math.MaxInt32)
		min32 := big.NewInt(math.MinInt32)
		if intVal.Cmp(max32) > 0 || intVal.Cmp(min32) < 0 {
			return diagnostic.NewError(i.Pos, "overflow: integer value %v too large for type %v", intVal, types.Int())
		}
		i.Type = types.Int()
	}

	return nil
}
