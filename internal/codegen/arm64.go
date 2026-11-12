package codegen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/chenota/acc/internal/ssa"
	"github.com/chenota/acc/internal/target"
)

// ARM64Op is an arm64 operation, independent of its operand size.
type ARM64Op int

const (
	ARM64MOV  ARM64Op = iota // register to register
	ARM64MOVZ                // a 16-bit immediate, zeroing the rest
	ARM64MOVN                // the inverse of a 16-bit immediate
	ARM64MOVK                // a 16-bit immediate, keeping the rest
	ARM64ADD
	ARM64SUB
	ARM64MUL
	ARM64SDIV
	ARM64NEG
	ARM64CMP
	ARM64CSET // 1 when the condition holds and 0 otherwise
	ARM64LDR
	ARM64STR
	ARM64LDP
	ARM64STP
	ARM64B
	ARM64CBZ
	ARM64CBNZ
	ARM64BL
	ARM64RET
	ARM64BRK
)

var arm64Mnemonics = map[ARM64Op]string{
	ARM64MOV:  "mov",
	ARM64MOVZ: "movz",
	ARM64MOVN: "movn",
	ARM64MOVK: "movk",
	ARM64ADD:  "add",
	ARM64SUB:  "sub",
	ARM64MUL:  "mul",
	ARM64SDIV: "sdiv",
	ARM64NEG:  "neg",
	ARM64CMP:  "cmp",
	ARM64CSET: "cset",
	ARM64LDR:  "ldr",
	ARM64STR:  "str",
	ARM64LDP:  "ldp",
	ARM64STP:  "stp",
	ARM64B:    "b",
	ARM64CBZ:  "cbz",
	ARM64CBNZ: "cbnz",
	ARM64BL:   "bl",
	ARM64RET:  "ret",
	ARM64BRK:  "brk",
}

// ARM64CondCode is a condition on the flags that the last comparison set
type ARM64CondCode int

const (
	ARM64EQ ARM64CondCode = iota
	ARM64NE
	ARM64LT // signed less
	ARM64LE // signed less or equal
	ARM64GT // signed greater
	ARM64GE // signed greater or equal
)

var arm64CondNames = map[ARM64CondCode]string{
	ARM64EQ: "eq",
	ARM64NE: "ne",
	ARM64LT: "lt",
	ARM64LE: "le",
	ARM64GT: "gt",
	ARM64GE: "ge",
}

type ARM64Inst struct {
	Op   ARM64Op
	Size int
	Args [3]ARM64Operand
}

type ARM64OperandKind int

const (
	ARM64None ARM64OperandKind = iota
	ARM64Reg
	ARM64Imm     // Imm shifted left by Shift bits
	ARM64Mem     // Offset bytes from the address in Reg
	ARM64PreIdx  // Offset bytes from the address in Reg, written back to Reg before the access
	ARM64PostIdx // the address in Reg, which moves Offset bytes after the access
	ARM64Sym
	ARM64Cond
)

type ARM64Operand struct {
	Kind   ARM64OperandKind
	Reg    target.Register
	Imm    int64
	Shift  int
	Offset int
	Sym    string
	Cond   ARM64CondCode
}

// String renders the instruction in GNU syntax.
func (i ARM64Inst) String() string {
	var ops []string
	for _, a := range i.Args {
		if a.Kind != ARM64None {
			ops = append(ops, a.text(i.Size))
		}
	}
	if len(ops) == 0 {
		return i.mnemonic()
	}
	return i.mnemonic() + " " + strings.Join(ops, ", ")
}

func (i ARM64Inst) mnemonic() string {
	m := arm64Mnemonics[i.Op]
	if i.Op == ARM64LDR || i.Op == ARM64STR {
		// narrow accesses name their width, while words and doublewords take it from the register
		m += map[int]string{1: "b", 2: "h"}[i.Size]
	}
	return m
}

func (o ARM64Operand) text(size int) string {
	switch o.Kind {
	case ARM64Reg:
		return arm64RegName(o.Reg, size)
	case ARM64Imm:
		if o.Shift != 0 {
			return fmt.Sprintf("#%d, lsl #%d", o.Imm, o.Shift)
		}
		return "#" + strconv.FormatInt(o.Imm, 10)
	case ARM64Mem:
		// an address is always full width
		if o.Offset == 0 {
			return "[" + arm64RegName(o.Reg, 8) + "]"
		}
		return fmt.Sprintf("[%s, #%d]", arm64RegName(o.Reg, 8), o.Offset)
	case ARM64PreIdx:
		return fmt.Sprintf("[%s, #%d]!", arm64RegName(o.Reg, 8), o.Offset)
	case ARM64PostIdx:
		return fmt.Sprintf("[%s], #%d", arm64RegName(o.Reg, 8), o.Offset)
	case ARM64Sym:
		return o.Sym
	case ARM64Cond:
		return arm64CondNames[o.Cond]
	}
	return ""
}

// arm64RegName names reg as a doubleword at size 8 and a word at anything narrower
func arm64RegName(reg target.Register, size int) string {
	if reg == target.RegXSP {
		if size == 8 {
			return "sp"
		}
		return "wsp"
	}
	if reg < target.RegX0 || reg > target.RegX30 {
		return fmt.Sprintf("!badreg(%d)", reg)
	}
	if size == 8 {
		return fmt.Sprintf("x%d", reg)
	}
	return fmt.Sprintf("w%d", reg)
}

// arm64 generates arm64 instructions.
type arm64 struct{}

var _ arch[ARM64Inst] = arm64{}

// arm64AddrScratch forms addresses too far from their base to reach directly
func arm64AddrScratch(e *emitter[ARM64Inst]) target.Register {
	return e.target.Scratch[0]
}

// arm64ValueScratch carries values between instructions
func arm64ValueScratch(e *emitter[ARM64Inst]) target.Register {
	return e.target.Scratch[1]
}

var (
	arm64FP = arm64Reg(target.RegX29)
	arm64LR = arm64Reg(target.RegX30)
	arm64SP = arm64Reg(target.RegXSP)
)

// generateARM64 lowers allocated SSA functions to arm64 instructions.
func generateARM64(program []*ssa.Func, s symbols) (*Program[ARM64Inst], error) {
	p, err := generate(arm64{}, s, program)
	if err != nil {
		return nil, err
	}

	for _, f := range p.Funcs {
		for _, b := range f.Blocks {
			b.Insts = arm64MovElim(b.Insts)
		}
	}

	return p, nil
}

func (arm64) prologue(e *emitter[ARM64Inst], f *ssa.Func) {
	e.emit(
		ARM64Inst{Op: ARM64STP, Size: 8, Args: [3]ARM64Operand{arm64FP, arm64LR, {Kind: ARM64PreIdx, Reg: target.RegXSP, Offset: -16}}},
		ARM64Inst{Op: ARM64MOV, Size: 8, Args: [3]ARM64Operand{arm64FP, arm64SP}},
	)

	arm64AddConst(e, target.RegXSP, target.RegXSP, -f.FrameSize())

	// the frame record anchors the saved registers, so they can go in once the frame exists
	for _, s := range f.SavedRegisters() {
		arm64Store(e, 8, s.Reg, s.Loc)
	}
}

func (arm64) epilogue(e *emitter[ARM64Inst], f *ssa.Func) {
	for _, s := range f.SavedRegisters() {
		arm64Load(e, 8, s.Reg, s.Loc)
	}

	// the frame pointer still marks where the frame record went, whatever the frame's size
	e.emit(
		ARM64Inst{Op: ARM64MOV, Size: 8, Args: [3]ARM64Operand{arm64SP, arm64FP}},
		ARM64Inst{Op: ARM64LDP, Size: 8, Args: [3]ARM64Operand{arm64FP, arm64LR, {Kind: ARM64PostIdx, Reg: target.RegXSP, Offset: 16}}},
		ARM64Inst{Op: ARM64RET},
	)
}

// arm64AddConst puts the address n bytes past src in dst
func arm64AddConst(e *emitter[ARM64Inst], dst, src target.Register, n int) {
	if n == 0 && dst == src {
		return
	}

	// immediates are unsigned, so stepping backwards takes a sub
	op := ARM64ADD
	if n < 0 {
		op, n = ARM64SUB, -n
	}

	if arm64AddImmFits(n) {
		e.emit(ARM64Inst{Op: op, Size: 8, Args: [3]ARM64Operand{arm64Reg(dst), arm64Reg(src), arm64Imm(int64(n))}})
		return
	}
	arm64MoveConst(e, 8, arm64AddrScratch(e), int64(n))
	e.emit(ARM64Inst{Op: op, Size: 8, Args: [3]ARM64Operand{arm64Reg(dst), arm64Reg(src), arm64Reg(arm64AddrScratch(e))}})
}

// arm64AddImmFits reports whether add and sub can take n as an unshifted 12-bit immediate
func arm64AddImmFits(n int) bool {
	return n >= 0 && n < 1<<12
}

func (arm64) terminator(e *emitter[ARM64Inst], f *ssa.Func, b, next *ssa.Block) {
	switch b.Kind {
	case ssa.BlockPlain:
		arm64BranchUnlessNext(e, f, b.Successors[0], next)
	case ssa.BlockIf:
		cond := b.Control[0]
		then, els := b.Successors[0], b.Successors[1]

		// only branch away from whichever successor comes next
		if then == next {
			e.emit(ARM64Inst{Op: ARM64CBZ, Size: cond.Type.Size(), Args: [3]ARM64Operand{arm64Reg(cond.Loc.Reg), arm64Sym(e.blockLabel(f, els))}})
			return
		}
		e.emit(ARM64Inst{Op: ARM64CBNZ, Size: cond.Type.Size(), Args: [3]ARM64Operand{arm64Reg(cond.Loc.Reg), arm64Sym(e.blockLabel(f, then))}})
		arm64BranchUnlessNext(e, f, els, next)
	case ssa.BlockTerminate:
		// the block's trap already stopped the program
	case ssa.BlockRet:
		// the last block falls straight into the epilogue
		if next != nil {
			e.emit(ARM64Inst{Op: ARM64B, Args: [3]ARM64Operand{arm64Sym(e.epilogueLabel(f))}})
		}
	}
}

func arm64BranchUnlessNext(e *emitter[ARM64Inst], f *ssa.Func, target, next *ssa.Block) {
	if target != next {
		e.emit(ARM64Inst{Op: ARM64B, Args: [3]ARM64Operand{arm64Sym(e.blockLabel(f, target))}})
	}
}

func (arm64) move(e *emitter[ARM64Inst], m move) {
	switch m.kind {
	case moveCopy:
		arm64Copy(e, m.size, m.dst, m.src)
	case moveSwap:
		// arm64 has no exchange, so one side waits in scratch; whole registers keep each value intact whatever its size
		e.emit(
			ARM64Inst{Op: ARM64MOV, Size: 8, Args: [3]ARM64Operand{arm64Reg(arm64ValueScratch(e)), arm64Reg(m.dst.Reg)}},
			ARM64Inst{Op: ARM64MOV, Size: 8, Args: [3]ARM64Operand{arm64Reg(m.dst.Reg), arm64Reg(m.src.Reg)}},
			ARM64Inst{Op: ARM64MOV, Size: 8, Args: [3]ARM64Operand{arm64Reg(m.src.Reg), arm64Reg(arm64ValueScratch(e))}},
		)
	}
}

var arm64BinaryOps = map[ssa.Op]ARM64Op{
	ssa.OpARM64ADD:  ARM64ADD,
	ssa.OpARM64SUB:  ARM64SUB,
	ssa.OpARM64MUL:  ARM64MUL,
	ssa.OpARM64SDIV: ARM64SDIV,
}

var arm64SetConds = map[ssa.Op]ARM64CondCode{
	ssa.OpARM64CSETEQ: ARM64EQ,
	ssa.OpARM64CSETNE: ARM64NE,
	ssa.OpARM64CSETLT: ARM64LT,
	ssa.OpARM64CSETLE: ARM64LE,
	ssa.OpARM64CSETGT: ARM64GT,
	ssa.OpARM64CSETGE: ARM64GE,
}

func (arm64) value(e *emitter[ARM64Inst], v *ssa.Value) error {
	size := v.Type.Size()

	switch v.Op {
	case ssa.OpARM64MOVconst:
		arm64MoveConstTo(e, size, v.Loc, arm64Literal(v))
	case ssa.OpStaticLoad:
		s := v.Slot()
		arm64Copy(e, size, v.Loc, ssa.NewMem(s.Loc.Reg, s.Loc.Offset+v.Offset))
	case ssa.OpStaticStore:
		s := v.Slot()
		arm64Copy(e, size, ssa.NewMem(s.Loc.Reg, s.Loc.Offset+v.Offset), v.Args[0].Loc)
	case ssa.OpARM64LDR:
		arm64Copy(e, size, v.Loc, ssa.NewMem(v.Args[0].Loc.Reg, v.Offset))
	case ssa.OpARM64STR:
		arm64Copy(e, size, ssa.NewMem(v.Args[1].Loc.Reg, v.Offset), v.Args[0].Loc)
	case ssa.OpARM64ADDslot:
		s := v.Slot()
		arm64AddConst(e, v.Loc.Reg, s.Loc.Reg, s.Loc.Offset+v.Offset)
	case ssa.OpARM64ADDconst:
		arm64AddConst(e, v.Loc.Reg, v.Args[0].Loc.Reg, v.Offset)
	case ssa.OpARM64ADD, ssa.OpARM64SUB, ssa.OpARM64MUL, ssa.OpARM64SDIV:
		e.emit(ARM64Inst{Op: arm64BinaryOps[v.Op], Size: arm64RegSize(size), Args: [3]ARM64Operand{arm64Reg(v.Loc.Reg), arm64Reg(v.Args[0].Loc.Reg), arm64Reg(v.Args[1].Loc.Reg)}})
	case ssa.OpARM64NEG:
		e.emit(ARM64Inst{Op: ARM64NEG, Size: arm64RegSize(size), Args: [3]ARM64Operand{arm64Reg(v.Loc.Reg), arm64Reg(v.Args[0].Loc.Reg)}})
	case ssa.OpARM64CSETEQ, ssa.OpARM64CSETNE, ssa.OpARM64CSETLT, ssa.OpARM64CSETLE, ssa.OpARM64CSETGT, ssa.OpARM64CSETGE:
		e.emit(
			// borrow the cmp's width from the operands
			ARM64Inst{Op: ARM64CMP, Size: arm64RegSize(v.Args[0].Type.Size()), Args: [3]ARM64Operand{arm64Reg(v.Args[0].Loc.Reg), arm64Reg(v.Args[1].Loc.Reg)}},
			ARM64Inst{Op: ARM64CSET, Size: arm64RegSize(size), Args: [3]ARM64Operand{arm64Reg(v.Loc.Reg), arm64Cond(arm64SetConds[v.Op])}},
		)
	case ssa.OpARM64BL:
		e.emit(ARM64Inst{Op: ARM64BL, Args: [3]ARM64Operand{arm64Sym(e.funcLabel(v.Callee()))}})
	case ssa.OpARM64BRK:
		e.emit(ARM64Inst{Op: ARM64BRK, Args: [3]ARM64Operand{arm64Imm(0)}})
	case ssa.OpCopy:
		arm64Copy(e, size, v.Loc, v.Args[0].Loc)
	case ssa.OpParam, ssa.OpResult, ssa.OpCallResult, ssa.OpClosurePtr, ssa.OpUnit, ssa.OpPhi:
		// placeholders values
	default:
		return fmt.Errorf("no code generator for op %s", v.Op)
	}
	return nil
}

// arm64Literal is the value a literal holds
func arm64Literal(v *ssa.Value) int64 {
	if b, ok := v.Value.(bool); ok {
		if b {
			return 1
		}
		return 0
	}
	return int64(v.Value.(int32))
}

// arm64Copy moves size bytes from src to dst, wherever each of them lives
func arm64Copy(e *emitter[ARM64Inst], size int, dst, src ssa.Location) {
	switch {
	case dst.Kind == ssa.LocRegister && src.Kind == ssa.LocRegister:
		e.emit(ARM64Inst{Op: ARM64MOV, Size: arm64RegSize(size), Args: [3]ARM64Operand{arm64Reg(dst.Reg), arm64Reg(src.Reg)}})
	case dst.Kind == ssa.LocRegister && src.Kind == ssa.LocMemory:
		arm64Load(e, size, dst.Reg, src)
	case dst.Kind == ssa.LocMemory && src.Kind == ssa.LocRegister:
		arm64Store(e, size, src.Reg, dst)
	case dst.Kind == ssa.LocMemory && src.Kind == ssa.LocMemory:
		// memory only moves through a register
		arm64Load(e, size, arm64ValueScratch(e), src)
		arm64Store(e, size, arm64ValueScratch(e), dst)
	}
}

func arm64Load(e *emitter[ARM64Inst], size int, dst target.Register, src ssa.Location) {
	e.emit(ARM64Inst{Op: ARM64LDR, Size: size, Args: [3]ARM64Operand{arm64Reg(dst), arm64Addr(e, size, src)}})
}

func arm64Store(e *emitter[ARM64Inst], size int, src target.Register, dst ssa.Location) {
	e.emit(ARM64Inst{Op: ARM64STR, Size: size, Args: [3]ARM64Operand{arm64Reg(src), arm64Addr(e, size, dst)}})
}

// arm64Addr is a memory operand that a size-byte load or store can reach loc through
func arm64Addr(e *emitter[ARM64Inst], size int, loc ssa.Location) ARM64Operand {
	if arm64OffsetFits(size, loc.Offset) {
		return ARM64Operand{Kind: ARM64Mem, Reg: loc.Reg, Offset: loc.Offset}
	}

	arm64MoveConst(e, 8, arm64AddrScratch(e), int64(loc.Offset))
	e.emit(ARM64Inst{Op: ARM64ADD, Size: 8, Args: [3]ARM64Operand{arm64Reg(arm64AddrScratch(e)), arm64Reg(loc.Reg), arm64Reg(arm64AddrScratch(e))}})
	return ARM64Operand{Kind: ARM64Mem, Reg: arm64AddrScratch(e)}
}

// arm64OffsetFits reports whether a size-byte load or store reaches offset straight from its base
func arm64OffsetFits(size, offset int) bool {
	if offset >= -256 && offset < 256 {
		return true
	}
	return offset >= 0 && offset%size == 0 && offset/size < 1<<12
}

// arm64MoveConstTo puts the constant n wherever loc is
func arm64MoveConstTo(e *emitter[ARM64Inst], size int, loc ssa.Location, n int64) {
	if loc.Kind == ssa.LocRegister {
		arm64MoveConst(e, size, loc.Reg, n)
		return
	}
	arm64MoveConst(e, size, arm64ValueScratch(e), n)
	arm64Store(e, size, arm64ValueScratch(e), loc)
}

// arm64MoveConst builds n in dst 16 bits at a time,
func arm64MoveConst(e *emitter[ARM64Inst], size int, dst target.Register, n int64) {
	regSize := arm64RegSize(size)
	chunks := regSize / 2

	piece := func(i int) int64 { return (n >> (16 * i)) & 0xffff }
	var zeros, ones int
	for i := range chunks {
		switch piece(i) {
		case 0:
			zeros++
		case 0xffff:
			ones++
		}
	}

	// start from whichever background most of the pieces already match
	inverted := ones > zeros
	background := int64(0)
	if inverted {
		background = 0xffff
	}

	var fill []int
	for i := range chunks {
		if piece(i) != background {
			fill = append(fill, i)
		}
	}
	// a constant that is all background still needs one instruction to set it
	if len(fill) == 0 {
		fill = []int{0}
	}

	for j, i := range fill {
		p := piece(i)
		switch {
		case j == 0 && inverted:
			e.emit(ARM64Inst{Op: ARM64MOVN, Size: regSize, Args: [3]ARM64Operand{arm64Reg(dst), arm64ShiftedImm(^p&0xffff, 16*i)}})
		case j == 0:
			e.emit(ARM64Inst{Op: ARM64MOVZ, Size: regSize, Args: [3]ARM64Operand{arm64Reg(dst), arm64ShiftedImm(p, 16*i)}})
		default:
			e.emit(ARM64Inst{Op: ARM64MOVK, Size: regSize, Args: [3]ARM64Operand{arm64Reg(dst), arm64ShiftedImm(p, 16*i)}})
		}
	}
}

// arm64RegSize is the width of the register holding a size-byte value, since nothing narrower than a word has one
func arm64RegSize(size int) int {
	if size == 8 {
		return 8
	}
	return 4
}

func arm64Reg(r target.Register) ARM64Operand {
	return ARM64Operand{Kind: ARM64Reg, Reg: r}
}

func arm64Imm(v int64) ARM64Operand {
	return ARM64Operand{Kind: ARM64Imm, Imm: v}
}

func arm64ShiftedImm(v int64, shift int) ARM64Operand {
	return ARM64Operand{Kind: ARM64Imm, Imm: v, Shift: shift}
}

func arm64Sym(name string) ARM64Operand {
	return ARM64Operand{Kind: ARM64Sym, Sym: name}
}

func arm64Cond(c ARM64CondCode) ARM64Operand {
	return ARM64Operand{Kind: ARM64Cond, Cond: c}
}

// arm64MovElim eliminates moves of a register onto itself
func arm64MovElim(insts []ARM64Inst) []ARM64Inst {
	return slices.DeleteFunc(insts, func(i ARM64Inst) bool {
		return i.Op == ARM64MOV && i.Args[0] == i.Args[1]
	})
}
