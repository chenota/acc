package semantic

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func Analyze(functions []*ir.Node) error {
	for _, f := range functions {
		if err := analyzeFunction(f, nil); err != nil {
			return err
		}
	}

	return nil
}

func analyzeStmt(n *ir.Node) error {
	switch n.Op {
	case ir.OpFunction:
		return analyzeFunction(n, nil)
	case ir.OpReturn:
		return analyzeReturn(n)
	default:
		return errors.New("unknown statement operation")
	}
}

func analyzeExpr(n *ir.Node, hint *types.Type) error {
	switch n.Op {
	case ir.OpFunction:
		return analyzeFunction(n, hint)
	case ir.OpInt:
		return analyzeInt(n, hint)
	default:
		return errors.New("unknown expression operation")
	}
}

func analyzeFunction(f *ir.Node, _ *types.Type) error {
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
		if err := analyzeStmt(s); err != nil {
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
	if err := analyzeExpr(e, expectedOut); err != nil {
		return err
	}

	// this check is redundant for now but will be useful in the future when we introduce more complexity
	if !types.Equal(e.Type, expectedOut) {
		return errors.New("return type does not match function signature")
	}

	return nil
}

func analyzeInt(i *ir.Node, hint *types.Type) error {
	if hint == nil {
		i.Type = types.UntypedInt()
		return nil
	}

	intVal := i.Val.(*big.Int)

	switch hint.Kind {
	case types.KInt32:
		max32 := big.NewInt(math.MaxInt32)
		min32 := big.NewInt(math.MinInt32)
		if intVal.Cmp(max32) > 0 || intVal.Cmp(min32) < 0 {
			return errors.New("int32 overflow")
		}
		i.Type = types.Int32()
	default:
		return fmt.Errorf("cannot use integer literal %v as type %v", intVal, hint)
	}

	return nil
}
