package ssa

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/types"
)

func GenSsa(program []*ir.Node) ([]*Func, error) {
	var compiledFuncs []*Func

	for _, f := range program {
		if f.Op != ir.OpFunction {
			return nil, errors.New("expect top-level function nodes")
		}

		// new state per function
		s := &state{}
		s.currFunc = &Func{Name: f.Sym.Name}

		s.currBlock = s.newBlock()

		for _, stmt := range f.List {
			if err := s.genStatement(stmt); err != nil {
				return nil, err
			}
		}

		compiledFuncs = append(compiledFuncs, s.currFunc)
	}

	return compiledFuncs, nil
}

func (s *state) genStatement(stmt *ir.Node) error {
	switch stmt.Op {
	case ir.OpReturn:
		retVal, err := s.genExpr(stmt.List[0])
		if err != nil {
			return err
		}
		s.setControlReturn(retVal)

		return nil
	default:
		return errors.New("unsupported statement op")
	}
}

func (s *state) genExpr(e *ir.Node) (*Value, error) {
	switch e.Op {
	case ir.OpInt:
		switch e.Type.Kind {
		case types.KInt32:
			v := s.newValue(OpConstInt32, types.Int32())
			v.AuxInt = e.Val.(*big.Int).Int64()
			return v, nil
		default:
			return nil, fmt.Errorf("unknown integer type: %v", e.Type)
		}
	default:
		return nil, errors.New("unsupported expression op")
	}
}

type state struct {
	blockId int
	valueId int

	currFunc  *Func
	currBlock *Block
}

func (s *state) newValue(op Op, t *types.Type) *Value {
	v := &Value{Id: s.valueId, Op: op, Type: t, Block: s.currBlock}
	s.valueId += 1
	s.currBlock.Values = append(s.currBlock.Values, v)
	return v
}

func (s *state) newBlock() *Block {
	b := &Block{Id: s.blockId}
	s.blockId += 1
	s.currFunc.Blocks = append(s.currFunc.Blocks, b)
	return b
}

func (s *state) setControlReturn(control *Value) {
	if s.currBlock == nil || s.currBlock.Kind != BlockUnset {
		return
	}
	s.currBlock.Kind = BlockRet
	s.currBlock.Control = control
}
