package ssa

import "github.com/chenota/acc/internal/types"

type Op int

const (
	OpConstInt32 Op = iota
	OpConstIntUntyped
)

type Value struct {
	Id    int
	Op    Op
	Type  *types.Type
	Block *Block

	Args []*Value

	Aux any
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
}
