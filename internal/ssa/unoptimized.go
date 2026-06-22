package ssa

import (
	"fmt"
	"math/big"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func buildFunc(n *ir.Node) (*Func, error) {
	if n.Op != ir.OpFunction {
		return nil, diagnostic.NewError("expected function node", n.Pos)
	}

	f := &Func{
		Name: n.Sym.Name,
	}

	b := &builder{targetFunc: f}

	entry := f.newBlock()
	f.Entry = entry
	b.currentBlock = entry

	for _, stmt := range n.List {
		if err := b.genStatement(stmt); err != nil {
			return nil, err
		}
	}

	return f, nil
}

type builder struct {
	targetFunc   *Func
	currentBlock *Block
}

func (b *builder) genStatement(stmt *ir.Node) error {
	switch stmt.Op {
	case ir.OpReturn:
		retVal, err := b.genExpr(stmt.List[0])
		if err != nil {
			return err
		}

		if b.currentBlock != nil && b.currentBlock.Kind == BlockUnset {
			b.currentBlock.Kind = BlockRet
			b.currentBlock.Control = retVal
		}

		return nil
	default:
		return diagnostic.NewError(fmt.Sprintf("unknown statement operation: %d", stmt.Op), stmt.Pos)
	}
}

func (b *builder) genExpr(expr *ir.Node) (*Value, error) {
	switch expr.Op {
	case ir.OpInt:
		return b.genInt(expr)
	case ir.OpPlus, ir.OpMinus, ir.OpTimes, ir.OpDiv:
		return b.genBop(expr)
	default:
		return nil, diagnostic.NewError(fmt.Sprintf("unknown expression operation: %d", expr.Op), expr.Pos)
	}
}

func (b *builder) genInt(expr *ir.Node) (*Value, error) {
	switch expr.Type.Kind {
	case types.KInt32:
		v := b.targetFunc.appendValue(OpLiteral, types.Int32(), b.currentBlock)
		v.Value = int32(expr.Val.(*big.Int).Int64())
		return v, nil
	default:
		return nil, diagnostic.NewError(fmt.Sprintf("unknown integer type: %v", expr.Type), expr.Pos)
	}
}

func (b *builder) genBop(expr *ir.Node) (*Value, error) {
	if len(expr.List) != 2 {
		return nil, diagnostic.NewError("binary operator without two operands", expr.Pos)
	}
	left := expr.List[0]
	right := expr.List[1]

	leftVal, err := b.genExpr(left)
	if err != nil {
		return nil, err
	}

	rightVal, err := b.genExpr(right)
	if err != nil {
		return nil, err
	}

	if expr.Type.IsConcreteNumeric() {
		v := b.targetFunc.appendValue(numericBopFrom(expr), expr.Type, b.currentBlock)
		v.Args = []*Value{leftVal, rightVal}
		return v, nil
	}

	return nil, diagnostic.NewError(fmt.Sprintf("cannot perform binary operation for type %v", expr.Type), expr.Pos)
}

func numericBopFrom(n *ir.Node) Op {
	switch n.Op {
	case ir.OpPlus:
		return OpAdd
	case ir.OpMinus:
		return OpSubtract
	case ir.OpTimes:
		return OpMultiply
	case ir.OpDiv:
		return OpDivide
	default:
		return OpUnknown
	}
}
