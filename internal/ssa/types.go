package ssa

import (
	"iter"
	"maps"
	"slices"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/types"
)

type Op int

const (
	OpUnknown Op = iota
	OpLiteral
	OpAlloca // virtual stack allocation - more of a placeholder for a location than an acutal value in its own right
	OpStore
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
	OpNegate
	OpCopy
	OpCall
	OpFuncRef    // the address of a function
	OpSignExtend // sign-extends the accumulator into the high register (cdq/cqo)
	OpPush
	OpPop
	OpParam // incoming function argument - more of a placeholder for a location than an acutal value in its own right
)

type Value struct {
	Id    int
	Op    Op
	Type  *types.Type
	Block *Block

	Args []*Value

	Value any

	Loc Location

	hints map[register.Register]int // hints stores the number of hints this value has per register

	Clobbers []register.Register // registers this op destroys
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

func (v *Value) ArgIndex(arg *Value) int {
	return slices.Index(v.Args, arg)
}

// NeedsRegister reports whether a value produces a result that occupies a physical register.
func (v *Value) NeedsRegister() bool {
	return v.Op != OpAlloca && v.Op != OpStore && v.Op != OpFuncRef
}

// RecordHint records that this value was hinted for a register
func (v *Value) RecordHint(r register.Register) {
	if v.hints == nil {
		v.hints = make(map[register.Register]int)
	}

	v.hints[r] += 1
}

// Hints returns a sequence of all hints this value has recorded in order of frequency, descending
func (v *Value) Hints() iter.Seq[register.Register] {
	type kv struct {
		key   register.Register
		value int
	}

	var hints []kv
	for k, v := range v.hints {
		hints = append(hints, kv{k, v})
	}

	slices.SortFunc(hints, func(a, b kv) int {
		if c := b.value - a.value; c != 0 {
			return c
		}
		// tie break on register number for determinism
		return int(a.key) - int(b.key)
	})

	var result []register.Register
	for _, hint := range hints {
		result = append(result, hint.key)
	}

	return slices.Values(result)
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

	valueId           int
	blockId           int
	frameSize         int // frameSize is the size of the function's general-purpose frame
	outgoingFrameSize int // outgoingFrameSize is the size of the function's frame reserved for outgoing arguments that don't fit in registers
}

// OrderedBlocks flattens a function's blocks using reverse post-order traversal
func (f *Func) OrderedBlocks() iter.Seq[*Block] {
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

	return slices.Values(order)
}

// OrderedValues returns all values in Func f in RPO order
func (f *Func) OrderedValues() iter.Seq[*Value] {
	var vals []*Value
	for b := range f.OrderedBlocks() {
		vals = append(vals, b.Values...)
	}
	return slices.Values(vals)
}

// UnorderedValues returns all values in Func f in an arbitrary order
func (f *Func) UnorderedValues() iter.Seq[*Value] {
	var vals []*Value
	for _, b := range f.Blocks {
		vals = append(vals, b.Values...)
	}
	return slices.Values(vals)
}

// Args returns all unique values used as arguments in function f
func (f *Func) Args() iter.Seq[*Value] {
	args := make(map[*Value]struct{})
	for v := range f.UnorderedValues() {
		for _, a := range v.Args {
			args[a] = struct{}{}
		}
	}
	return maps.Keys(args)
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

// Label returns the function's assembly symbol.
// TODO: this will need expanded for anonymous functions.
func (f *Func) Label() string {
	return "_" + f.Name
}

// redirectUses points every reference to old at new - does not touch the instruction stream itself.
func (f *Func) redirectUses(old, new *Value) {
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

// replaceValue old with new in the instruction stream and redirects uses of old to new.
func (f *Func) replaceValue(old, new *Value) {
	for _, block := range f.Blocks {
		if i := block.indexOf(old); i >= 0 {
			block.Values[i] = new
		}
	}

	f.redirectUses(old, new)
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

func (f *Func) maxOutgoingSize() int {
	var max int
	for v := range f.UnorderedValues() {
		// Args[0] is the callee so ignore it to count # of arguments
		if v.Op == OpCall && len(v.Args)-1 > len(register.Args) {
			// each outgoing stack slot uses 8 bytes
			outgoingSize := stackSlotSize * (len(v.Args) - 1 - len(register.Args))
			if outgoingSize > max {
				max = outgoingSize
			}
		}
	}
	return max
}

// UsedRegisters returns the set of physical registers assigned to values in f.
func (f *Func) UsedRegisters() register.Mask {
	var m register.Mask
	for v := range f.UnorderedValues() {
		if v.Loc.Kind == LocRegister {
			m = m.Include(v.Loc.Reg)
		}
	}
	return m
}

type LocationKind int

const (
	LocNone LocationKind = iota
	LocRegister
	LocMemory // a byte offset from a base register
)

type Location struct {
	Kind   LocationKind
	Reg    register.Register // Reg is the value's register for LocRegister, or the base register it is addressed off of for LocMemory
	Offset int               // Offset is the byte offset from Reg for LocMemory
}

func NewReg(reg register.Register) Location {
	return Location{
		Kind: LocRegister,
		Reg:  reg,
	}
}

// NewMem addresses a location at a byte offset from a base register
func NewMem(base register.Register, offset int) Location {
	return Location{
		Kind:   LocMemory,
		Reg:    base,
		Offset: offset,
	}
}

// NewFrame addresses a slot in the current frame
func NewFrame(offset int) Location {
	return NewMem(register.RegBP, offset)
}

// NewOutgoing addresses a slot in this function's outgoing argument area
func NewOutgoing(offset int) Location {
	return NewMem(register.RegSP, offset)
}
