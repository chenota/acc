package ssa

import (
	"iter"
	"slices"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/target"
	"github.com/chenota/acc/internal/types"
)

type Value struct {
	Id    int
	Op    Op
	Type  *types.Type
	Block *Block

	Args  []*Value
	users []*Value
	ctrl  []*Block // the blocks whose Control reads this value

	Value any

	Offset int // byte offset from the slot or pointer this value addresses.

	Loc Location
}

func (v *Value) IsCall() bool {
	return v.Op.info().call
}

// CallArgs returns the operands of a call that are ABI arguments
func (v *Value) CallArgs() []*Value {
	if v.Op.info().closureCall {
		// code pointer and env come before arguments
		return v.Args[2:]
	}
	return v.Args
}

func (v *Value) IsUnaryOp() bool {
	return v.Op.info().unary
}

func (v *Value) IsBinaryOp() bool {
	return v.Op.info().binary
}

func (v *Value) IsComparison() bool {
	return v.Op.info().comparison
}

func (v *Value) IsConstant() bool {
	return v.Op.info().constant
}

func (v *Value) IsAssociativeOp() bool {
	return v.Op.info().associative
}

// NeedsRegister reports whether a value produces a result that occupies a physical register.
func (v *Value) NeedsRegister() bool {
	return !(v.Op.info().noResult || v.Type.IsSingleton())
}

func (v *Value) Rematerializable() bool {
	return v.Op.info().rematerializable
}

// Slot is the frame slot this value reads or writes, or nil if it does not touch one.
func (v *Value) Slot() *Slot {
	s, _ := v.Value.(*Slot)
	return s
}

// Callee is the function an OpCall targets.
func (v *Value) Callee() *Func {
	f, _ := v.Value.(*Func)
	return f
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
	Func   *Func

	Successors   []*Block
	Predecessors []*Block

	Control []*Value
}

func (b *Block) indexOf(v *Value) int {
	for i, value := range b.Values {
		if value == v {
			return i
		}
	}
	return -1
}

// firstUse is the index of the first value in b naming slot.
func (b *Block) firstUse(slot *Slot) int {
	for i, v := range b.Values {
		if v.Slot() == slot {
			return i
		}
	}

	return len(b.Values)
}

// phiIndex reports the index that separates phi values from regular values
func (b *Block) phiIndex() int {
	var i int
	for _, v := range b.Values {
		if v.Op != OpPhi {
			break
		}
		i += 1
	}
	return i
}

// PhiArgsFrom yields each of b's phis with the operand it takes on the edge from pred
func (b *Block) PhiArgsFrom(pred *Block) iter.Seq2[*Value, *Value] {
	i := slices.Index(b.Predecessors, pred)
	return func(yield func(*Value, *Value) bool) {
		for _, phi := range b.Values[:b.phiIndex()] {
			if !yield(phi, phi.Args[i]) {
				return
			}
		}
	}
}

type Func struct {
	name   string
	Blocks []*Block
	Entry  *Block
	Slots  []*Slot

	Target *target.Target

	Params  []*types.Type
	Results []*types.Type

	valueId int
	blockId int

	// memoized parameter escape summary
	escapes []paramSummary
	// set while f's summary is being computed
	// TODO: I hate this here has to be a better way
	escapeInProgress bool
}

func newFunc(name string, sig *types.Type) *Func {
	return &Func{
		name: name,
		// kind of hacky but wrapping sig.Params in a tuple type lets us get the leaves w/o a loop in here
		Params:  slices.Collect(iterutil.Second(types.Tuple(sig.Params()).Leaves())),
		Results: slices.Collect(iterutil.Second(sig.Result().Leaves())),
	}
}

// newExternFunc declares a function whose body lives outside the module
func newExternFunc(name string, sig *types.Type, escapes []paramSummary) *Func {
	f := newFunc(name, sig)
	f.escapes = escapes
	return f
}

// Slot is a variable's home in the stack frame.
type Slot struct {
	Sym  *ir.Sym
	Type *types.Type
	Loc  Location
}

// newSlot reserves a frame slot in f. The slot has no location until layoutFrame runs.
func (f *Func) newSlot(sym *ir.Sym, t *types.Type) *Slot {
	s := &Slot{Sym: sym, Type: t}
	f.Slots = append(f.Slots, s)
	return s
}

// paramValues return f's parameter SSA values in order
func (f *Func) paramValues() []*Value {
	vals := make([]*Value, len(f.Params))
	for v := range f.UnorderedValues() {
		if v.Op == OpParam {
			vals[v.Value.(int)] = v
		}
	}
	return vals
}

// heapSeeds gives the values in f that reach the heap directly
func (f *Func) heapSeeds() []seed {
	var seeds []seed
	for v := range f.UnorderedValues() {
		switch v.Op {
		case OpStore:
			seeds = append(seeds, seed{valueNode(v.Args[0]), 0})
		case OpClosureCall:
			// unknown callee, so every argument is assumed to escape
			// TODO: turn as many closures into static calls as possible in an optimization step
			// environment itself can't escape so start at a surplus
			seeds = append(seeds, seed{valueNode(v.Args[1]), 1})
			for _, arg := range v.CallArgs() {
				seeds = append(seeds, seed{valueNode(arg), 0})
			}
		case OpStaticCall:
			// use callee's summary to figure out which paramters reach the heap and at what level
			for i, s := range v.Callee().escapeSummary() {
				if s.heap != nil {
					seeds = append(seeds, seed{valueNode(v.Args[i]), *s.heap})
				}
			}
		}
	}
	return seeds
}

// resultValues returns all results in f with index i
func (f *Func) resultValues(i int) []*Value {
	var vals []*Value
	for v := range f.UnorderedValues() {
		if v.Op == OpResult {
			if k, ok := v.Value.(int); ok && k == i {
				vals = append(vals, v)
			}
		}
	}
	return vals
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

func (f *Func) insertValueFront(op Op, t *types.Type, b *Block) *Value {
	return f.insertValueAt(0, op, t, b)
}

// insertValueAt splices a new value into b at index i.
// TODO: Use this function as a helper in more funcs
func (f *Func) insertValueAt(i int, op Op, t *types.Type, b *Block) *Value {
	newVal := f.newValue(op, t, b)
	b.Values = slices.Insert(b.Values, i, newVal)

	return newVal
}

func (f *Func) newBlock() *Block {
	b := &Block{Id: f.blockId, Func: f}
	f.blockId += 1
	f.Blocks = append(f.Blocks, b)
	return b
}

// splitEdge puts a new plain block on the edge from p to b
func (f *Func) splitEdge(p, b *Block) *Block {
	e := f.newBlock()
	e.Kind = BlockPlain
	e.Predecessors = []*Block{p}
	e.Successors = []*Block{b}

	p.Successors[slices.Index(p.Successors, b)] = e
	b.Predecessors[slices.Index(b.Predecessors, p)] = e

	return e
}

func (f *Func) IsMain() bool {
	return f.name == "main"
}

// Name returns the function's source-level name.
func (f *Func) Name() string {
	return f.name
}

// StackAdjustment is the number of bytes the function prologue must subtract from rsp.
func (f *Func) StackAdjustment() int {
	pushBytes := f.savedRegBytes()
	region := max(pushBytes, f.localsSize()) + f.maxOutgoingSize()
	return ((region + 15) &^ 15) - pushBytes
}

// savedRegBytes is the number of bytes the prologue pushes for callee-saved registers.
func (f *Func) savedRegBytes() int {
	return f.SavedRegisters().Count() * 8
}

// localsSize is the size in bytes of the frame's slot area
func (f *Func) localsSize() int {
	var size int
	for _, s := range f.Slots {
		size = max(size, -s.Loc.Offset)
	}
	return size
}

func (f *Func) maxOutgoingSize() int {
	var most int
	for v := range f.UnorderedValues() {
		if !v.IsCall() {
			continue
		}
		nArgs := len(v.CallArgs())
		slots := f.outgoingArgs().overflow(nArgs) + f.incomingResults(nArgs).overflow(v.Type.LeafCount())
		most = max(most, stackSlotSize*slots)
	}
	return most
}

// paramLeaves is how many leaves the signature's parameters flatten into
func (f *Func) paramLeaves() int {
	var n int
	for v := range f.UnorderedValues() {
		if v.Op == OpParam {
			n += 1
		}
	}
	return n
}

// UsedRegisters returns the set of physical registers assigned to values in f.
func (f *Func) UsedRegisters() target.Mask {
	var m target.Mask
	for v := range f.UnorderedValues() {
		if v.Loc.Kind == LocRegister {
			m = m.Include(v.Loc.Reg)
		}
	}
	return m
}

func (f *Func) SavedRegisters() target.Mask {
	return f.UsedRegisters().Intersect(f.Target.CalleeSaved)
}

// nearestDominatingBlocks gives each used slot the nearest block dominating all of its uses.
func (f *Func) nearestDominatingBlocks() map[*Slot]*Block {
	dom := computeDominators(f)
	homes := make(map[*Slot]*Block)
	for v := range f.UnorderedValues() {
		s := v.Slot()
		if s == nil {
			continue
		}
		if home, ok := homes[s]; ok {
			homes[s] = dom.nearestCommon(home, v.Block)
		} else {
			homes[s] = v.Block
		}
	}
	return homes
}

// SlotValues returns every value that names the given frame slot
func (f *Func) SlotValues(s *Slot) iter.Seq[*Value] {
	return func(yield func(*Value) bool) {
		for v := range f.UnorderedValues() {
			if v.Slot() == s && !yield(v) {
				return
			}
		}
	}
}

type Loop struct {
	Header *Block              // entry point into the loop, and where back edges point to
	Outer  *Loop               // immediate parent of this loop
	Blocks map[*Block]struct{} // every block in the loop
}

func (l *Loop) contains(b *Block) bool {
	_, ok := l.Blocks[b]
	return ok
}

func (f *Func) Loops() map[*Block]*Loop {
	dom := computeDominators(f)
	order := slices.Collect(f.OrderedBlocks())
	innermost := make(map[*Block]*Loop)

	// walk backwards to find inner loops first
	for _, h := range slices.Backward(order) {
		var work []*Block
		// search for predecessors that are dominated by h, since that indicates a loop
		for _, p := range h.Predecessors {
			if dom.dominates(h, p) {
				work = append(work, p)
			}
		}
		if len(work) == 0 {
			continue
		}

		l := &Loop{Header: h, Blocks: map[*Block]struct{}{h: {}}}
		// h dominates all blocks in work, so it must dominate their predecessors, and so on...
		for len(work) > 0 {
			b := work[len(work)-1]
			work = work[:len(work)-1]
			if _, ok := l.Blocks[b]; ok {
				continue
			}
			l.Blocks[b] = struct{}{}
			work = append(work, b.Predecessors...)
		}

		for b := range l.Blocks {
			in, ok := innermost[b]
			if !ok {
				// block doesn't belong to a loop, so this loop must be its innermost
				innermost[b] = l
				continue
			}
			// block already belongs to a loop, which must sit inside this one, so wrap l around the known outermost loop this block belongs to
			for in.Outer != nil && in.Outer != l {
				in = in.Outer
			}
			in.Outer = l
		}
	}

	return innermost
}

type LocationKind int

const (
	LocNone LocationKind = iota
	LocRegister
	LocMemory // a byte offset from a base register
)

type Location struct {
	Kind   LocationKind
	Reg    target.Register // Reg is the value's register for LocRegister, or the base register it is addressed off of for LocMemory
	Offset int             // Offset is the byte offset from Reg for LocMemory
}

func NewReg(reg target.Register) Location {
	return Location{
		Kind: LocRegister,
		Reg:  reg,
	}
}

// NewMem addresses a location at a byte offset from a base register
func NewMem(base target.Register, offset int) Location {
	return Location{
		Kind:   LocMemory,
		Reg:    base,
		Offset: offset,
	}
}

// frameLoc addresses a slot in f's frame
func (f *Func) frameLoc(offset int) Location {
	return NewMem(f.Target.FramePointer, offset)
}

// outgoingLoc addresses a slot in f's outgoing argument area
func (f *Func) outgoingLoc(offset int) Location {
	return NewMem(f.Target.StackPointer, offset)
}
