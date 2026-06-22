package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/types"
)

type Op int

const (
	OpUnknown Op = iota
	OpLiteral
	OpAlloca
	OpLoad
	OpStore
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
)

type Value struct {
	Id    int
	Op    Op
	Type  *types.Type
	Block *Block

	Args []*Value

	Value any

	Loc Location
}

func (v *Value) IsBinaryOp() bool {
	return v.Op == OpAdd || v.Op == OpSubtract || v.Op == OpMultiply || v.Op == OpDivide
}

func (v *Value) IsConstant() bool {
	return v.Op == OpLiteral
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

	Successors   []*Block
	Predecessors []*Block

	Control *Value
}

func (b *Block) OrderedValues() []*Value {
	var order []*Value
	visited := make(map[int]struct{})

	var visit func(*Value)
	visit = func(v *Value) {
		if _, ok := visited[v.Id]; ok {
			return
		}
		visited[v.Id] = struct{}{}

		for _, arg := range v.Args {
			visit(arg)
		}

		order = append(order, v)
	}

	visit(b.Control)

	return order
}

type Func struct {
	Name   string
	Blocks []*Block
	Entry  *Block

	valueId   int
	blockId   int
	frameSize int
}

// OrderedBlocks flattens a function's blocks using reverse post-order traversal
func (f *Func) OrderedBlocks() []*Block {
	var order []*Block
	visited := make(map[int]struct{})

	var visit func(*Block)
	visit = func(b *Block) {
		if _, ok := visited[b.Id]; ok {
			return
		}
		visited[b.Id] = struct{}{}

		for _, succ := range b.Successors {
			visit(succ)
		}

		order = append(order, b)
	}

	visit(f.Entry)

	slices.Reverse(order)

	return order
}

func (f *Func) values() []*Value {
	var vals []*Value
	for _, b := range f.OrderedBlocks() {
		vals = append(vals, b.OrderedValues()...)
	}
	return vals
}

func (f *Func) newValue(op Op, t *types.Type, b *Block) *Value {
	v := &Value{Id: f.valueId, Op: op, Type: t, Block: b}
	f.valueId += 1
	b.Values = append(b.Values, v)
	return v
}

func (f *Func) newBlock() *Block {
	b := &Block{Id: f.blockId}
	f.blockId += 1
	f.Blocks = append(f.Blocks, b)
	return b
}

func (f *Func) IsMain() bool {
	return f.Name == "main"
}

func (f *Func) Label() string {
	return "_" + f.Name
}

func (f *Func) substituteValue(old, new *Value) {
	for _, block := range f.Blocks {
		for _, value := range block.Values {
			for i := range value.Args {
				if value.Args[i] == old {
					value.Args[i] = new
				}
			}
		}

		if block.Control == old {
			block.Control = new
		}
	}
}

func (f *Func) FrameSize() int {
	return f.frameSize
}

type LocationKind int

const (
	LocNone LocationKind = iota
	LocRegister
	LocStack
)

type Location struct {
	Kind   LocationKind
	Reg    register.Register
	Offset int // negative byte offset from rbp
}

func NewReg(reg register.Register) Location {
	return Location{
		Kind: LocRegister,
		Reg:  reg,
	}
}

func NewStack(offset int) Location {
	return Location{
		Kind:   LocStack,
		Offset: offset,
	}
}
