package semantic

import (
	"errors"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func Analyze(functions []*ir.Node) error {
	for _, f := range functions {
		if err := analyzeNode(f); err != nil {
			return err
		}
	}

	return nil
}

func analyzeNode(n *ir.Node) error {
	if n == nil {
		return errors.New("nil node")
	}

	switch n.Op {
	case ir.OpFunction:
		return analyzeFunction(n)
	case ir.OpReturn:
		return analyzeReturn(n)
	case ir.OpInt:
		return analyzeInt(n)
	default:
		return errors.New("unknown operation")
	}
}

func analyzeFunction(f *ir.Node) error {
	// set own type
	var paramTypes []*types.Type
	for _, p := range f.Signature.Params {
		paramTypes = append(paramTypes, p.Type)
	}
	f.Type = types.Function(paramTypes, f.Signature.Result.Type)

	// register own symbol
	f.Sym = &ir.Sym{
		Name: f.Name,
		Def:  f,
		Type: f.Type,
	}

	// analyze types of body statements
	for _, s := range f.List {
		if err := analyzeNode(s); err != nil {
			return err
		}
	}

	return nil
}

func analyzeReturn(r *ir.Node) error {
	// grab first function we can find in the AST
	currentFunc := r.Predecessor(ir.OpFunction)

	// we expect a return to appear in a function
	if currentFunc == nil {
		return errors.New("return outside of function definition")
	}
	expectedOut := currentFunc.Type.Output

	// determine type of sub-expression
	e := r.List[0]
	if err := analyzeNode(e); err != nil {
		return err
	}

	// is e cast-able to return type we want?
	if !canCast(e, expectedOut) {
		return errors.New("cannot cast expression to function return type")
	}

	// inject a type conversion node
	if !types.Equal(e.Type, expectedOut) {
		r.List[0] = &ir.Node{
			Parent: r,
			Op:     ir.OpConv,
			Type:   expectedOut,
			List:   []*ir.Node{e},
			Pos:    e.Pos,
		}
		e.Parent = r.List[0]
	}

	return nil
}

func analyzeInt(i *ir.Node) error {
	// int nodes always start out as untyped
	i.Type = types.UntypedInt()
	return nil
}

func canCast(n *ir.Node, t *types.Type) bool {
	if types.Equal(n.Type, t) {
		return true
	}
	if n.Type.Kind == types.KUntypedInt && t.Kind == types.KInt32 {
		return true
	}
	return false
}
