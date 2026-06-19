package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/types"
)

type Op int

const (
	OpUnknown Op = iota
	OpConstInt32
	OpStoreReg
	OpLoadReg
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

	AuxInt int64

	Loc Location
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
	spillSlot int
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

func (f *Func) allocateSpill() int {
	f.spillSlot += 1
	return f.spillSlot - 1
}

// StackSize returns a 16-byte aligned value representing how large Func's stack is.
func (f *Func) StackSize() int64 {
	return int64(((f.spillSlot * 8) / 16) * 16)
}

func (f *Func) IsMain() bool {
	return f.Name == "main"
}

func (f *Func) Label() string {
	return "_" + f.Name
}

type LocationKind int

const (
	LocNone LocationKind = iota
	LocRegister
	LocStack
)

type Location struct {
	Kind LocationKind
	Reg  register.Register
	Slot int
}

func NewReg(reg register.Register) Location {
	return Location{
		Kind: LocRegister,
		Reg:  reg,
	}
}

func NewStack(slot int) Location {
	return Location{
		Kind: LocStack,
		Slot: slot,
	}
}
