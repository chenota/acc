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
}
