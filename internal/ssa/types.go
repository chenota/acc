package ssa

import "github.com/chenota/acc/internal/types"

type Op int

const (
	OpUnknown Op = iota
	OpConstInt32
)

type Value struct {
	Id    int
	Op    Op
	Type  *types.Type
	Block *Block

	Args []*Value

	AuxInt int64
}

type BlockKind int

const (
	BlockUnset BlockKind = iota
	BlockPlain
	BlockIf
	BlockRet
)

type Block struct {
	Id     int
	Kind   BlockKind
	Values []*Value

	Control *Value
}

type Func struct {
	Name   string
	Blocks []*Block
	Entry  *Block

	valueId int
	blockId int
}

func (f *Func) newValue(op Op, t *types.Type, b *Block) *Value {
	v := &Value{Id: f.valueId, Op: op, Type: t, Block: b}
	f.valueId += 1
	return v
}

func (f *Func) newBlock() *Block {
	b := &Block{Id: f.blockId}
	f.blockId += 1
	f.Blocks = append(f.Blocks, b)
	return b
}
