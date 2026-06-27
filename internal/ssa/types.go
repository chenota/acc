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
	OpNegate
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

func (v *Value) IsUnaryOp() bool {
	return v.Op == OpNegate
}

func (v *Value) IsBinaryOp() bool {
	return v.Op == OpAdd || v.Op == OpSubtract || v.Op == OpMultiply || v.Op == OpDivide
}

func (v *Value) IsConstant() bool {
	return v.Op == OpLiteral
}

func (v *Value) IsAssociativeOp() bool {
	return v.Op == OpAdd || v.Op == OpMultiply
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

func (b *Block) indexOf(v *Value) int {
	for i, value := range b.Values {
		if value == v {
			return i
		}
	}
	return -1
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
		vals = append(vals, b.Values...)
	}
	return vals
}

func (f *Func) newValue(op Op, t *types.Type, b *Block) *Value {
	v := &Value{Id: f.valueId, Op: op, Type: t, Block: b}
	f.valueId += 1
	return v
}

func (f *Func) appendValue(op Op, t *types.Type, b *Block) *Value {
	newVal := f.newValue(op, t, b)
	b.Values = append(b.Values, newVal)
	return newVal
}

func (f *Func) insertValueBefore(v *Value, op Op, t *types.Type, b *Block) *Value {
	blockIdx := b.indexOf(v)
	if blockIdx < 0 {
		return nil
	}

	newVal := f.newValue(op, t, b)
	b.Values = slices.Insert(b.Values, blockIdx, newVal)

	return newVal
}

func (f *Func) insertValueAfter(v *Value, op Op, t *types.Type, b *Block) *Value {
	blockIdx := b.indexOf(v)
	if blockIdx < 0 {
		return nil
	}

	newVal := f.newValue(op, t, b)
	b.Values = slices.Insert(b.Values, blockIdx+1, newVal)

	return newVal
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
		for i, v := range block.Values {
			if v == old {
				block.Values[i] = new
			}
		}

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

func (f *Func) removeValue(v *Value) {
	// filter v out of each block's values list and control value
	for _, block := range f.Blocks {
		// stupid go does not have a filter function so this is what we're doing
		var n int
		for _, value := range block.Values {
			if value != v {
				block.Values[n] = value
				n += 1
			}
		}
		block.Values = block.Values[:n]

		if block.Control == v {
			block.Control = nil
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
