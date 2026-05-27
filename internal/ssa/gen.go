package ssa

import (
	"errors"
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
	if s == nil {
		return errors.New("nil node")
	}

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
	if e == nil {
		return nil, errors.New("nil node")
	}

	switch e.Op {
	case ir.OpInt:
		v := s.newValue(OpConstIntUntyped, types.UntypedInt())
		v.Aux = e.Val // e.Val stores a big.Int for this kind of node
		return v, nil
	case ir.OpConv:
		argVal, err := s.genExpr(e.List[0])
		if err != nil {
			return nil, err
		}
		if e.Type.Kind == types.KInt32 && argVal.Op == OpConstIntUntyped {
			v := s.newValue(OpConstInt32, types.Int32())
			rawBig, ok := argVal.Aux.(*big.Int)
			if !ok {
				return nil, errors.New("untyped int op missing correct aux type")
			}
			v.Aux = int32(rawBig.Int64())
			return v, nil
		}
		return nil, errors.New("unsupported type conversion")
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
	if s == nil || s.currBlock == nil || s.currBlock.Kind != BlockUnset {
		return
	}
	s.currBlock.Kind = BlockRet
	s.currBlock.Control = control
}
