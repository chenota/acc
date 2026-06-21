package semantic

import (
	"fmt"
	"math"
	"math/big"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func Analyze(functions []*ir.Node) error {
	globalScope := ir.NewTable()

	for _, f := range functions {
		if err := analyzeFunction(globalScope, f); err != nil {
			return err
		}
	}

	return nil
}

func analyzeStmt(scope *ir.Table, n *ir.Node) error {
	switch n.Op {
	case ir.OpFunction:
		return analyzeFunction(scope, n)
	case ir.OpReturn:
		return analyzeReturn(scope, n)
	case ir.OpDeclaration:
		return analyzeDeclaration(scope, n)
	case ir.OpAssignment:
		return analyzeAssignment(scope, n)
	default:
		return diagnostic.NewError(fmt.Sprintf("unknown statement operation: %d", n.Op), n.Pos)
	}
}

func analyzeAssignment(scope *ir.Table, n *ir.Node) error {
	// need an existing symbol for this ident
	existingSym := scope.Sym(n.Name)
	if existingSym == nil {
		return diagnostic.NewError(fmt.Sprintf("variable used before declaration: %v", n.Name), n.Pos)
	}

	n.Sym = existingSym

	// analyze the expression with hint of existing type
	if len(n.List) != 1 {
		return diagnostic.NewError("variable assignment missing expression", n.Pos)
	}
	if err := analyzeExpr(scope, n.List[0], n.Sym.Type); err != nil {
		return err
	}

	// make sure the expression and wanted type match
	if !types.Equal(n.Sym.Type, n.List[0].Type) {
		return diagnostic.NewError(fmt.Sprintf("variable declaration with mismatched types: want %v, got %v", n.Sym.Type, n.List[0].Type), n.Pos)
	}

	return nil
}

func analyzeDeclaration(scope *ir.Table, n *ir.Node) error {
	if len(n.List) != 2 {
		return diagnostic.NewError("variable declaration missing components", n.Pos)
	}
	typeNode := n.List[0]
	e := n.List[1]

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
		hint = defaultType
		if err := analyzeExpr(scope, e, hint); err != nil {
			return err
		}
	}

	// wanted type must equal got type
	if !types.Equal(hint, e.Type) {
		return diagnostic.NewError(fmt.Sprintf("variable declaration with mismatched types: want %v, got %v", hint, e.Type), n.Pos)
	}

	// register self in scope; will get nil if variable already exists in scope
	sym := scope.Register(n.Name, e.Type)
	if sym == nil {
		return diagnostic.NewError(fmt.Sprintf("variable re-declared: %v", n.Name), n.Pos)
	}
	n.Sym = sym

	return nil
}

func analyzeExpr(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	switch n.Op {
	case ir.OpFunction:
		return analyzeFunction(scope, n)
	case ir.OpInt:
		return analyzeInt(n, hint)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return analyzeBop(scope, n, hint)
	case ir.OpIdent:
		return analyzeIdent(scope, n)
	default:
		return diagnostic.NewError(fmt.Sprintf("unknown expression operation: %d", n.Op), n.Pos)
	}
}

func analyzeIdent(scope *ir.Table, n *ir.Node) error {
	// need an existing symbol for this ident
	existingSym := scope.Sym(n.Name)
	if existingSym == nil {
		return diagnostic.NewError(fmt.Sprintf("variable used before declaration: %v", n.Name), n.Pos)
	}

	n.Type = existingSym.Type
	n.Sym = existingSym

	return nil
}

func analyzeBop(scope *ir.Table, n *ir.Node, hint *types.Type) error {
	// extract left and right operands
	if len(n.List) != 2 {
		return diagnostic.NewError("binary operator without two operands", n.Pos)
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
		return diagnostic.NewError(fmt.Sprintf("binary operation with mismatched types: %v and %v", leftType, rightType), n.Pos)
	}

	// finally, assign bop node to the agreed-upon type
	n.Type = leftType

	return nil
}

func analyzeFunction(scope *ir.Table, f *ir.Node) error {
	// set own type
	var paramTypes []*types.Type
	for _, p := range f.Signature.Params {
		paramTypes = append(paramTypes, p.Type)
	}
	f.Type = types.Function(paramTypes, f.Signature.Result.Type)

	// register self onto scope
	sym := scope.Register(f.Name, f.Type)
	if sym == nil {
		return diagnostic.NewError(fmt.Sprintf("symbol '%s' already declared", f.Name), f.Pos)
	}
	f.Sym = sym

	// need a child scope for function body
	funScope := scope.NewChild()

	// analyze types of body statements
	for _, s := range f.List {
		if err := analyzeStmt(funScope, s); err != nil {
			return err
		}
	}

	return nil
}

func analyzeReturn(scope *ir.Table, r *ir.Node) error {
	// grab first function we can find in the AST
	currentFunc := r.Predecessor(ir.OpFunction)

	// we expect a return to appear in a function
	if currentFunc == nil {
		return diagnostic.NewError("return statement appears outside of a function definition", r.Pos)
	}
	expectedOut := currentFunc.Type.Output

	// determine type of sub-expression
	e := r.List[0]
	if err := analyzeExpr(scope, e, expectedOut); err != nil {
		return err
	}

	// this check is redundant for now but will be useful in the future when we introduce more complexity
	if !types.Equal(e.Type, expectedOut) {
		return diagnostic.NewError(fmt.Sprintf("return value type does not match type of function signature. expected %v, got %v", expectedOut, e.Type), e.Pos)
	}

	return nil
}

func analyzeInt(i *ir.Node, hint *types.Type) error {
	i.Type = types.UntypedInt()

	intVal := i.Val.(*big.Int)

	if types.Equal(hint, types.Int32()) {
		max32 := big.NewInt(math.MaxInt32)
		min32 := big.NewInt(math.MinInt32)
		if intVal.Cmp(max32) > 0 || intVal.Cmp(min32) < 0 {
			return diagnostic.NewError(fmt.Sprintf("overflow: integer value %v too large for type %v", intVal, types.Int32()), i.Pos)
		}
		i.Type = types.Int32()
	}

	return nil
}
