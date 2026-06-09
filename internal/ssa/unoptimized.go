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
		switch expr.Type.Kind {
		case types.KInt32:
			v := b.targetFunc.newValue(OpConstInt32, types.Int32(), b.currentBlock)
			v.AuxInt = expr.Val.(*big.Int).Int64()
			return v, nil
		default:
			return nil, diagnostic.NewError(fmt.Sprintf("unknown integer type: %v", expr.Type), expr.Pos)
		}
	default:
		return nil, diagnostic.NewError(fmt.Sprintf("unknown expression operation: %d", expr.Op), expr.Pos)
	}
}
