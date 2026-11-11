package codegen

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/chenota/acc/internal/ssa"
	"github.com/chenota/acc/internal/target"
)

// X64Op is an x86-64 operation, independent of its operand size.
type X64Op int

const (
	X64MOV X64Op = iota
	X64LEA
	X64ADD
	X64SUB
	X64IMUL
	X64IDIV
	X64SignExtendA // sign-extends the A register into D ahead of a divide
	X64NEG
	X64XOR
	X64CMP
	X64TEST
	X64SETE
	X64SETNE
	X64SETL
	X64SETLE
	X64SETG
	X64SETGE
	X64JMP
	X64JE
	X64JNE
	X64CALL
	X64RET
	X64PUSH
	X64POP
	X64XCHG
	X64UD2
)

var x64Mnemonics = map[X64Op]string{
	X64MOV:   "mov",
	X64LEA:   "lea",
	X64ADD:   "add",
	X64SUB:   "sub",
	X64IMUL:  "imul",
	X64IDIV:  "idiv",
	X64NEG:   "neg",
	X64XOR:   "xor",
	X64CMP:   "cmp",
	X64TEST:  "test",
	X64SETE:  "sete",
	X64SETNE: "setne",
	X64SETL:  "setl",
	X64SETLE: "setle",
	X64SETG:  "setg",
	X64SETGE: "setge",
	X64JMP:   "jmp",
	X64JE:    "je",
	X64JNE:   "jne",
	X64CALL:  "call",
	X64RET:   "ret",
	X64PUSH:  "push",
	X64POP:   "pop",
	X64XCHG:  "xchg",
	X64UD2:   "ud2",
}

// x64Suffixed holds the operations whose AT&T mnemonic names their operand size
var x64Suffixed = map[X64Op]bool{
	X64MOV: true, X64LEA: true, X64ADD: true, X64SUB: true, X64IMUL: true, X64IDIV: true,
	X64NEG: true, X64XOR: true, X64CMP: true, X64TEST: true, X64PUSH: true, X64POP: true, X64XCHG: true,
}

// X64Inst is one x86-64 instruction, whose register operands are all Size bytes wide.
type X64Inst struct {
	Op   X64Op
	Size int
	Src  X64Operand
	Dst  X64Operand
}

type X64OperandKind int

const (
	X64None X64OperandKind = iota
	X64Reg
	X64Imm
	X64Mem      // Offset bytes from the address in Reg
	X64Sym      // a label or symbol, as a jump or call target
	X64RipRel   // a label's address, relative to the instruction pointer
	X64Indirect // a jump or call target held in Reg
)

type X64Operand struct {
	Kind   X64OperandKind
	Reg    target.Register
	Imm    int64
	Offset int
	Sym    string
}

// String renders the instruction in AT&T syntax.
func (i X64Inst) String() string {
	op := i.mnemonic()
	src, dst := i.Src.text(i.Size), i.Dst.text(i.Size)
	switch {
	case dst == "":
		return op
	case src == "":
		return op + " " + dst
	default:
		return op + " " + src + ", " + dst
	}
}

func (i X64Inst) mnemonic() string {
	if i.Op == X64SignExtendA {
		if i.Size == 8 {
			return "cqo"
		}
		return "cdq"
	}
	m := x64Mnemonics[i.Op]
	if x64Suffixed[i.Op] {
		m += x64SizeSuffix(i.Size)
	}
	return m
}

func x64SizeSuffix(size int) string {
	switch size {
	case 1:
		return "b"
	case 2:
		return "w"
	case 4:
		return "l"
	default:
		return "q"
	}
}

func (o X64Operand) text(size int) string {
	switch o.Kind {
	case X64Reg:
		return x64RegName(o.Reg, size)
	case X64Imm:
		return "$" + strconv.FormatInt(o.Imm, 10)
	case X64Mem:
		return fmt.Sprintf("%d(%s)", o.Offset, x64RegName(o.Reg, 8))
	case X64Sym:
		return o.Sym
	case X64RipRel:
		return o.Sym + "(%rip)"
	case X64Indirect:
		// an address is always full width
		return "*" + x64RegName(o.Reg, 8)
	}
	return ""
}

var x64ClassicRegs = map[target.Register][4]string{
	target.RegA:  {"al", "ax", "eax", "rax"},
	target.RegB:  {"bl", "bx", "ebx", "rbx"},
	target.RegC:  {"cl", "cx", "ecx", "rcx"},
	target.RegD:  {"dl", "dx", "edx", "rdx"},
	target.RegSI: {"sil", "si", "esi", "rsi"},
	target.RegDI: {"dil", "di", "edi", "rdi"},
	target.RegSP: {"spl", "sp", "esp", "rsp"},
	target.RegBP: {"bpl", "bp", "ebp", "rbp"},
}

// x64RegName names reg at the given width; anything unnameable comes out as text no assembler accepts
func x64RegName(reg target.Register, size int) string {
	width := map[int]int{1: 0, 2: 1, 4: 2, 8: 3}
	w, ok := width[size]
	if !ok {
		return fmt.Sprintf("%%!badsize(%d,%d)", reg, size)
	}
	if names, ok := x64ClassicRegs[reg]; ok {
		return "%" + names[w]
	}
	if reg >= target.Reg8 && reg <= target.Reg15 {
		return fmt.Sprintf("%%r%d%s", reg, [4]string{"b", "w", "d", ""}[w])
	}
	return fmt.Sprintf("%%!badreg(%d)", reg)
}

// x64 generates x86-64 instructions.
type x64 struct{}

var _ arch[X64Inst] = x64{}

var (
	basePointer  = x64Reg(target.RegBP)
	stackPointer = x64Reg(target.RegSP)
)

// generateX64 lowers allocated SSA functions to x86-64 instructions.
func generateX64(program []*ssa.Func, s symbols) (*Program[X64Inst], error) {
	p, err := generate(x64{}, s, program)
	if err != nil {
		return nil, err
	}

	for _, f := range p.Funcs {
		for _, b := range f.Blocks {
			b.Insts = movElim(b.Insts)
		}
	}

	return p, nil
}

func (x64) prologue(e *emitter[X64Inst], f *ssa.Func) {
	e.emit(
		X64Inst{Op: X64PUSH, Size: 8, Dst: basePointer},
		X64Inst{Op: X64MOV, Size: 8, Src: stackPointer, Dst: basePointer},
	)

	for _, s := range f.SavedRegisters() {
		e.emit(X64Inst{Op: X64PUSH, Size: 8, Dst: x64Reg(s.Reg)})
	}

	if adjust := x64StackAdjust(f); adjust > 0 {
		e.emit(X64Inst{Op: X64SUB, Size: 8, Src: x64Imm(int64(adjust)), Dst: stackPointer})
	}
}

func (x64) epilogue(e *emitter[X64Inst], f *ssa.Func) {
	if adjust := x64StackAdjust(f); adjust > 0 {
		e.emit(X64Inst{Op: X64ADD, Size: 8, Src: x64Imm(int64(adjust)), Dst: stackPointer})
	}

	// restore in reverse order so each pop mirrors its push
	for _, s := range slices.Backward(f.SavedRegisters()) {
		e.emit(X64Inst{Op: X64POP, Size: 8, Dst: x64Reg(s.Reg)})
	}

	e.emit(
		X64Inst{Op: X64POP, Size: 8, Dst: basePointer},
		X64Inst{Op: X64RET},
	)
}

// x64StackAdjust is how far the stack pointer moves past the saved registers' pushes to make room for the frame
func x64StackAdjust(f *ssa.Func) int {
	return f.FrameSize() - 8*len(f.SavedRegisters())
}

func (x64) terminator(e *emitter[X64Inst], f *ssa.Func, b, next *ssa.Block) {
	switch b.Kind {
	case ssa.BlockPlain:
		x64JumpUnlessNext(e, f, b.Successors[0], next)
	case ssa.BlockIf:
		cond := toOperand(b.Control[0])
		then, els := b.Successors[0], b.Successors[1]
		e.emit(X64Inst{Op: X64TEST, Size: b.Control[0].Type.Size(), Src: cond, Dst: cond})

		// only branch away from whichever successor comes next
		if then == next {
			// if cond is zero (that's what je measures) then jump to else
			e.emit(X64Inst{Op: X64JE, Dst: x64Sym(e.blockLabel(f, els))})
			return
		}
		// jump to then if cond is nonzero
		e.emit(X64Inst{Op: X64JNE, Dst: x64Sym(e.blockLabel(f, then))})
		// otherwise, fall through and jump to else (unless it's directly next)
		x64JumpUnlessNext(e, f, els, next)
	case ssa.BlockTerminate:
		// the block's trap already stopped the program
	case ssa.BlockRet:
		// the last block falls straight into the epilogue
		if next != nil {
			e.emit(X64Inst{Op: X64JMP, Dst: x64Sym(e.epilogueLabel(f))})
		}
	}
}

func x64JumpUnlessNext(e *emitter[X64Inst], f *ssa.Func, target, next *ssa.Block) {
	if target != next {
		e.emit(X64Inst{Op: X64JMP, Dst: x64Sym(e.blockLabel(f, target))})
	}
}

func (x64) move(e *emitter[X64Inst], m move) {
	switch m.kind {
	case moveCopy:
		e.emit(X64Inst{Op: X64MOV, Size: m.size, Src: locOperand(m.src), Dst: locOperand(m.dst)})
	case moveSwap:
		// trading whole registers keeps each value intact whatever its size
		e.emit(X64Inst{Op: X64XCHG, Size: 8, Src: x64Reg(m.src.Reg), Dst: x64Reg(m.dst.Reg)})
	}
}

var x64SetOps = map[ssa.Op]X64Op{
	ssa.OpX64SETE:  X64SETE,
	ssa.OpX64SETNE: X64SETNE,
	ssa.OpX64SETL:  X64SETL,
	ssa.OpX64SETLE: X64SETLE,
	ssa.OpX64SETG:  X64SETG,
	ssa.OpX64SETGE: X64SETGE,
}

var x64BinaryOps = map[ssa.Op]X64Op{
	ssa.OpX64ADD:  X64ADD,
	ssa.OpX64SUB:  X64SUB,
	ssa.OpX64IMUL: X64IMUL,
}

func (x64) value(e *emitter[X64Inst], v *ssa.Value) error {
	size := v.Type.Size()

	switch v.Op {
	case ssa.OpX64MOVconst:
		e.emit(X64Inst{Op: X64MOV, Size: size, Src: literalImmediate(v), Dst: toOperand(v)})
	case ssa.OpStaticLoad:
		e.emit(X64Inst{Op: X64MOV, Size: size, Src: slotOperand(v.Slot(), v.Offset), Dst: toOperand(v)})
	case ssa.OpStaticStore:
		e.emit(X64Inst{Op: X64MOV, Size: size, Src: toOperand(v.Args[0]), Dst: slotOperand(v.Slot(), v.Offset)})
	case ssa.OpX64LEAQslot:
		e.emit(X64Inst{Op: X64LEA, Size: 8, Src: slotOperand(v.Slot(), v.Offset), Dst: toOperand(v)})
	case ssa.OpX64LEAQ:
		e.emit(X64Inst{Op: X64LEA, Size: 8, Src: indirect(v.Args[0], v.Offset), Dst: toOperand(v)})
	case ssa.OpX64MOVstore:
		e.emit(X64Inst{Op: X64MOV, Size: size, Src: toOperand(v.Args[0]), Dst: indirect(v.Args[1], v.Offset)})
	case ssa.OpX64MOVload:
		e.emit(X64Inst{Op: X64MOV, Size: size, Src: indirect(v.Args[0], v.Offset), Dst: toOperand(v)})
	case ssa.OpX64ADD, ssa.OpX64SUB, ssa.OpX64IMUL:
		e.emit(
			X64Inst{Op: X64MOV, Size: size, Src: toOperand(v.Args[0]), Dst: toOperand(v)},
			X64Inst{Op: x64BinaryOps[v.Op], Size: size, Src: toOperand(v.Args[1]), Dst: toOperand(v)},
		)
	case ssa.OpX64IDIV:
		// sign-extend the dividend into rdx before idiv divides rdx:rax
		e.emit(
			X64Inst{Op: X64SignExtendA, Size: size},
			X64Inst{Op: X64IDIV, Size: size, Dst: toOperand(v.Args[1])},
			X64Inst{Op: X64MOV, Size: size, Src: x64Reg(target.RegA), Dst: toOperand(v)},
		)
	case ssa.OpX64SETE, ssa.OpX64SETNE, ssa.OpX64SETL, ssa.OpX64SETLE, ssa.OpX64SETG, ssa.OpX64SETGE:
		// TODO: Split this into separate cmp and setcc operations for more efficient branching
		e.emit(
			// borrow the cmp op's width from operands
			X64Inst{Op: X64CMP, Size: v.Args[0].Type.Size(), Src: toOperand(v.Args[1]), Dst: toOperand(v.Args[0])},
			X64Inst{Op: x64SetOps[v.Op], Size: size, Dst: toOperand(v)},
		)
	case ssa.OpX64NEG:
		e.emit(
			X64Inst{Op: X64MOV, Size: size, Src: toOperand(v.Args[0]), Dst: toOperand(v)},
			X64Inst{Op: X64NEG, Size: size, Dst: toOperand(v)},
		)
	case ssa.OpX64XORconst:
		// xors its operand with the constant lowering left in v
		e.emit(
			X64Inst{Op: X64MOV, Size: size, Src: toOperand(v.Args[0]), Dst: toOperand(v)},
			X64Inst{Op: X64XOR, Size: size, Src: x64Imm(int64(v.Value.(int32))), Dst: toOperand(v)},
		)
	case ssa.OpCopy:
		e.emit(X64Inst{Op: X64MOV, Size: size, Src: toOperand(v.Args[0]), Dst: toOperand(v)})
	case ssa.OpX64CALLstatic:
		e.emit(X64Inst{Op: X64CALL, Dst: x64Sym(e.funcLabel(v.Callee()))})
	case ssa.OpX64CALLclosure:
		// jumps to the code pointer the closure carries alongside its environment
		e.emit(X64Inst{Op: X64CALL, Dst: X64Operand{Kind: X64Indirect, Reg: v.Args[0].Loc.Reg}})
	case ssa.OpX64UD2:
		e.emit(X64Inst{Op: X64UD2})
	case ssa.OpX64LEAQrip:
		// materializes a function's address as a value
		e.emit(X64Inst{Op: X64LEA, Size: 8, Src: X64Operand{Kind: X64RipRel, Sym: e.funcLabel(v.Callee())}, Dst: toOperand(v)})
	case ssa.OpParam, ssa.OpResult, ssa.OpCallResult, ssa.OpClosurePtr, ssa.OpUnit, ssa.OpPhi:
		// placeholders values
	default:
		return fmt.Errorf("no code generator for op %s", v.Op)
	}
	return nil
}

// literalImmediate is the immediate operand holding a literal's value
func literalImmediate(v *ssa.Value) X64Operand {
	if b, ok := v.Value.(bool); ok {
		if b {
			return x64Imm(1)
		}
		return x64Imm(0)
	}
	return x64Imm(int64(v.Value.(int32)))
}

// slotOperand is the memory operand addressing a slot in the current frame.
func slotOperand(s *ssa.Slot, offset int) X64Operand {
	return X64Operand{Kind: X64Mem, Reg: s.Loc.Reg, Offset: s.Loc.Offset + offset}
}

func indirect(ptr *ssa.Value, offset int) X64Operand {
	return X64Operand{Kind: X64Mem, Reg: ptr.Loc.Reg, Offset: offset}
}

func toOperand(v *ssa.Value) X64Operand {
	return locOperand(v.Loc)
}

// locOperand addresses the value held at loc
func locOperand(loc ssa.Location) X64Operand {
	switch loc.Kind {
	case ssa.LocRegister:
		return x64Reg(loc.Reg)
	case ssa.LocMemory:
		return X64Operand{Kind: X64Mem, Reg: loc.Reg, Offset: loc.Offset}
	}
	return X64Operand{}
}

func x64Reg(r target.Register) X64Operand {
	return X64Operand{Kind: X64Reg, Reg: r}
}

func x64Imm(v int64) X64Operand {
	return X64Operand{Kind: X64Imm, Imm: v}
}

func x64Sym(name string) X64Operand {
	return X64Operand{Kind: X64Sym, Sym: name}
}

// movElim eliminates redundant mov instructions
func movElim(insts []X64Inst) []X64Inst {
	return slices.DeleteFunc(insts, func(i X64Inst) bool {
		return i.Op == X64MOV && i.Src == i.Dst
	})
}
