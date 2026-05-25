package analyze

import (
	"errors"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func AnalyzeNode(n *ir.Node) error {
	c := &checker{}
	return c.analyzeNode(n)
}

type checker struct {
	currentFunc *ir.Node
}

func (c *checker) analyzeNode(n *ir.Node) error {
	if n == nil {
		return errors.New("nil node")
	}

	switch n.Op {
	case ir.OpFunction:
		return c.analyzeFunction(n)
	case ir.OpReturn:
		return c.analyzeReturn(n)
	case ir.OpInt:
		return c.analyzeInt(n)
	default:
		return errors.New("unknown operation")
	}
}

func (c *checker) analyzeFunction(f *ir.Node) error {
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

	// cache the previous function context
	oldFunc := c.currentFunc
	c.currentFunc = f
	defer func() { c.currentFunc = oldFunc }()

	// analyze types of body statements
	for _, s := range f.List {
		if err := c.analyzeNode(s); err != nil {
			return err
		}
	}

	return nil
}

func (c *checker) analyzeReturn(r *ir.Node) error {
	// we expect a return to appear in a function
	if c.currentFunc == nil {
		return errors.New("return outside of function definition")
	}
	expectedOut := c.currentFunc.Type.Output

	// determine type of sub-expression
	e := r.List[0]
	if err := c.analyzeNode(e); err != nil {
		return err
	}

	// is e cast-able to return type we want?
	if !c.canCast(e, expectedOut) {
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

func (c *checker) analyzeInt(i *ir.Node) error {
	// int nodes always start out as untyped
	i.Type = types.UntypedInt()
	return nil
}

func (c *checker) canCast(n *ir.Node, t *types.Type) bool {
	if types.Equal(n.Type, t) {
		return true
	}
	if n.Type.Kind == types.KUntypedInt && t.Kind == types.KInt32 {
		return true
	}
	return false
}
