package semantic

import (
	"math"
	"math/big"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func Analyze(functions []*ir.Node) error {
	globalScope := ir.NewTable()

	// register every global function's signature first
	for _, f := range functions {
		if err := registerFunction(globalScope, f); err != nil {
			return err
		}
	}

	for _, f := range functions {
		if err := analyzeFunction(globalScope, f); err != nil {
			return err
		}
	}

	return nil
}

func analyzeStmt(scope *ir.Table, n *ir.Node) error {
	switch n.Op {
	case ir.OpReturn:
		return analyzeReturn(scope, n)
	case ir.OpDeclaration:
		return analyzeDeclaration(scope, n)
	case ir.OpAssignment:
		return analyzeAssignment(scope, n)
	case ir.OpPlusEq, ir.OpMinusEq, ir.OpDivEq, ir.OpTimesEq:
		// assignment operators have same structure and use same typing rules as regular assignment
		return analyzeAssignment(scope, n)
	default:
		return diagnostic.NewError(n.Pos, "unknown statement operation: %d", n.Op)
	}
}

func analyzeAssignment(scope *ir.Table, n *ir.Node) error {
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
	if err := analyzeExpr(scope, target, nil); err != nil {
		return err
	}

	// analyze the expression with hint of the target's type
	if err := analyzeExpr(scope, e, target.Type); err != nil {
		return err
	}

	// make sure the expression and target type match
	if !types.Equal(target.Type, e.Type) {
		return diagnostic.NewError(n.Pos, "variable assignment with mismatched types: want %v, got %v", target.Type, e.Type)
	}

	return nil
}

func analyzeDeclaration(scope *ir.Table, n *ir.Node) error {
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

	if err := analyzeExpr(scope, e, hint); err != nil {
		return err
	}

	// we need a concrete type at this point to resolve any unknowns. must re-analyze with hint if type changes.
	defaultType := e.Type.ToDefault()
	if !types.Equal(defaultType, e.Type) {
		if err := analyzeExpr(scope, e, defaultType); err != nil {
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
	sym := scope.Register(nameNode.Ident(), e.Type)
	if sym == nil {
		return diagnostic.NewError(nameNode.Pos, "variable re-declared: %v", nameNode.Ident())
	}
	n.Sym = sym

	return nil
}

func analyzeExpr(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	switch n.Op {
	case ir.OpInt:
		return analyzeInt(n, hint)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return analyzeBop(scope, n, hint)
	case ir.OpIdent:
		return analyzeIdent(scope, n)
	case ir.OpNegate:
		return analyzeNegate(scope, n, hint)
	case ir.OpCall:
		return analyzeCall(scope, n)
	default:
		return diagnostic.NewError(n.Pos, "unknown expression operation: %d", n.Op)
	}
}

func analyzeCall(scope *ir.Table, n *ir.Node) error {
	if len(n.List) < 1 {
		return diagnostic.NewError(n.Pos, "call without a callee")
	}

	// analyze the expression being called
	callee := n.List[0]
	if err := analyzeExpr(scope, callee, nil); err != nil {
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

		if err := analyzeExpr(scope, arg, param); err != nil {
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

func analyzeNegate(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	if len(n.List) != 1 {
		return diagnostic.NewError(n.Pos, "negation without an argument")
	}

	// analyze sub-expression with hint
	e := n.List[0]
	analyzeExpr(scope, e, hint)

	// steal type from sub-expression
	n.Type = e.Type

	return nil
}

func analyzeIdent(scope *ir.Table, n *ir.Node) error {
	// need an existing symbol for this ident
	existingSym := scope.Sym(n.Ident())
	if existingSym == nil {
		return diagnostic.NewError(n.Pos, "variable used before declaration: %v", n.Ident())
	}

	n.Type = existingSym.Type
	n.Sym = existingSym

	return nil
}

func analyzeBop(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	// extract left and right operands
	if len(n.List) != 2 {
		return diagnostic.NewError(n.Pos, "binary operator without two operands")
	}
	left := n.List[0]
	right := n.List[1]

	// figure out types of left and right operands given context
	if err := analyzeExpr(scope, left, hint); err != nil {
		return err
	}
	if err := analyzeExpr(scope, right, hint); err != nil {
		return err
	}
	leftType := left.Type
	rightType := right.Type

	// attempt to resolve flexible types
	switch {
	case leftType.IsUntypedNumeric() && rightType.IsConcreteNumeric():
		if err := analyzeExpr(scope, left, rightType); err != nil {
			return err
		}
	case leftType.IsConcreteNumeric() && rightType.IsUntypedNumeric():
		if err := analyzeExpr(scope, right, leftType); err != nil {
			return err
		}
	}
	leftType = left.Type
	rightType = right.Type

	// types must be equal
	if !types.Equal(leftType, rightType) {
		return diagnostic.NewError(n.Pos, "binary operation with mismatched types: %v and %v", leftType, rightType)
	}

	// finally, assign bop node to the agreed-upon type
	n.Type = leftType

	return nil
}

func registerFunction(scope *ir.Table, f *ir.Node) error {
	// resolve parameter types and set own type
	var paramTypes []*types.Type
	for _, p := range f.Signature.Params {
		if err := analyzeParam(p); err != nil {
			return err
		}
		paramTypes = append(paramTypes, p.Type)
	}
	f.Type = types.Function(paramTypes, f.Signature.Result.Type)

	// register self onto scope
	name := f.Signature.Name
	sym := scope.Register(name.Ident(), f.Type)
	if sym == nil {
		return diagnostic.NewError(name.Pos, "symbol '%s' already declared", name.Ident())
	}
	f.Sym = sym

	return nil
}

func analyzeFunction(scope *ir.Table, f *ir.Node) error {
	// need a child scope for function body
	funScope := scope.NewChild()

	// register parameters into the function scope so the body can reference them
	for _, p := range f.Signature.Params {
		pName := p.List[0]
		sym := funScope.Register(pName.Ident(), p.Type)
		if sym == nil {
			return diagnostic.NewError(pName.Pos, "parameter '%s' already declared", pName.Ident())
		}
		p.Sym = sym
	}

	// analyze types of body statements
	for _, s := range f.List {
		if err := analyzeStmt(funScope, s); err != nil {
			return err
		}
	}

	return nil
}

func analyzeParam(p *ir.Node) error {
	if len(p.List) != 2 {
		return diagnostic.NewError(p.Pos, "parameter missing type")
	}

	// pull the resolved type from the type node (List[1], after the name) up into the param node
	p.Type = p.List[1].Type

	return nil
}

func analyzeReturn(scope *ir.Table, r *ir.Node) error {
	// grab first function we can find in the AST
	currentFunc := r.Predecessor(ir.OpFunction)

	// we expect a return to appear in a function
	if currentFunc == nil {
		return diagnostic.NewError(r.Pos, "return statement appears outside of a function definition")
	}
	expectedOut := currentFunc.Type.Result()

	// determine type of sub-expression
	e := r.List[0]
	if err := analyzeExpr(scope, e, expectedOut); err != nil {
		return err
	}

	// this check is redundant for now but will be useful in the future when we introduce more complexity
	if !types.Equal(e.Type, expectedOut) {
		return diagnostic.NewError(e.Pos, "return value type does not match type of function signature. expected %v, got %v", expectedOut, e.Type)
	}

	return nil
}

func analyzeInt(i *ir.Node, hint *types.Type) error {
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
