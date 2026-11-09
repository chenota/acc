package codegen

import (
	"errors"
	"fmt"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/ssa"
	"github.com/chenota/acc/internal/target"
)

var (
	basePointer  = Arg{Kind: KRegister, Reg: target.RegBP, Value: 8}
	stackPointer = Arg{Kind: KRegister, Reg: target.RegSP, Value: 8}
)

func GenerateProgram(program []*ssa.Func) ([]Inst, error) {
	var insts []Inst

	insts = append(insts, Inst{
		Op: ".text",
	})

	var mainFunc *ssa.Func
	for _, f := range program {
		if f.IsMain() {
			mainFunc = f
			break
		}
	}

	if mainFunc == nil {
		return nil, errors.New("program has no main function")
	}

	for _, f := range program {
		fn, err := generateFunction(f)
		if err != nil {
			return nil, err
		}
		insts = append(insts, fn...)
	}

	// remove all redundant mov instructions
	insts = movElim(insts)

	return insts, nil
}

func generateFunction(f *ssa.Func) ([]Inst, error) {
	var insts []Inst

	insts = append(insts,
		Inst{
			Op:   ".globl",
			Dest: text(funcLabel(f)),
		},
		label(funcLabel(f)),
		Inst{
			Op:   "pushq",
			Dest: basePointer,
		},
		Inst{
			Op:   "movq",
			Src1: stackPointer,
			Dest: basePointer,
		},
	)

	// Save the callee-saved registers this function uses above the local frame.
	saved := f.SavedRegisters().All()
	for reg := range saved {
		insts = append(insts, Inst{Op: "pushq", Dest: register64(reg)})
	}

	stackAdjust := f.StackAdjustment()
	if stackAdjust > 0 {
		insts = append(insts, Inst{
			Op:   "subq",
			Src1: immediate(int32(stackAdjust)),
			Dest: stackPointer,
		})
	}

	blocks := slices.Collect(f.OrderedBlocks())
	for i, b := range blocks {
		// next is the block laid out right after b
		var next *ssa.Block
		if i+1 < len(blocks) {
			next = blocks[i+1]
		}
		block, err := generateBlock(f, b, next)
		if err != nil {
			return nil, err
		}
		insts = append(insts, block...)
	}

	// every return block ends up here
	insts = append(insts, label(epilogueLabel(f)))

	if stackAdjust > 0 {
		insts = append(insts, Inst{
			Op:   "addq",
			Src1: immediate(int32(stackAdjust)),
			Dest: stackPointer,
		})
	}

	// restore in reverse order so each pop mirrors its push
	for reg := range iterutil.Reverse(saved) {
		insts = append(insts, Inst{Op: "popq", Dest: register64(reg)})
	}

	insts = append(insts, Inst{
		Op:   "popq",
		Dest: basePointer,
	})

	insts = append(insts, Inst{Op: "ret"})

	return insts, nil
}

func generateBlock(f *ssa.Func, b *ssa.Block, next *ssa.Block) ([]Inst, error) {
	insts := []Inst{label(blockLabel(f, b))}

	for _, v := range b.Values {
		vi, err := generateValue(v)
		if err != nil {
			return nil, err
		}
		insts = append(insts, vi...)
	}

	return append(insts, generateTerminator(f, b, next)...), nil
}

// generateTerminator gives control to b's successors
func generateTerminator(f *ssa.Func, b *ssa.Block, next *ssa.Block) []Inst {
	switch b.Kind {
	case ssa.BlockPlain:
		// the successor's phis take their operands from b on the way in
		insts := generateEdgeMoves(b, b.Successors[0])
		return append(insts, jumpUnlessNext(f, b.Successors[0], next)...)
	case ssa.BlockIf:
		cond := toArg(b.Control[0])
		then, els := b.Successors[0], b.Successors[1]
		insts := []Inst{{Op: "testb", Src1: cond, Dest: cond}}

		// only branch away from whichever successor comes next
		if then == next {
			// if cond is zero (that's what je measures) then jump to else
			return append(insts, Inst{Op: "je", Dest: text(blockLabel(f, els))})
		}
		// jump to then if cond is nonzero
		insts = append(insts, Inst{Op: "jne", Dest: text(blockLabel(f, then))})
		// otherwise, fall through and jump to else (unless it's directly next)
		return append(insts, jumpUnlessNext(f, els, next)...)
	case ssa.BlockRet:
		// the last block falls straight into the epilogue
		if next == nil {
			return nil
		}
		return []Inst{{Op: "jmp", Dest: text(epilogueLabel(f))}}
	}
	return nil
}

// generateEdgeMoves puts each operand pred hands succ's phis where that phi lives
func generateEdgeMoves(pred, succ *ssa.Block) []Inst {
	var pending []move
	for phi, arg := range succ.PhiArgsFrom(pred) {
		if phi.Loc.Kind == ssa.LocNone || phi.Loc == arg.Loc {
			continue
		}
		pending = append(pending, move{kind: moveCopy, dst: phi.Loc, src: arg.Loc, size: phi.Type.Size()})
	}

	var insts []Inst
	for _, m := range sequentialize(pending) {
		switch m.kind {
		case moveCopy:
			insts = append(insts, Inst{
				Op:   movOp(m.size),
				Src1: locArg(m.src, m.size),
				Dest: locArg(m.dst, m.size),
			})
		case moveSwap:
			// trading whole registers keeps each value intact whatever its size
			insts = append(insts, Inst{
				Op:   "xchgq",
				Src1: register64(m.src.Reg),
				Dest: register64(m.dst.Reg),
			})
		}
	}

	return insts
}

func jumpUnlessNext(f *ssa.Func, target *ssa.Block, next *ssa.Block) []Inst {
	if target == next {
		return nil
	}
	return []Inst{{Op: "jmp", Dest: text(blockLabel(f, target))}}
}

// generateValue emits v's instructions
func generateValue(v *ssa.Value) ([]Inst, error) {
	var insts []Inst

	switch v.Op {
	case ssa.OpLiteral:
		insts = append(insts, generateLiteral(v))
	case ssa.OpStaticLoad:
		insts = append(insts, generateStaticLoad(v))
	case ssa.OpStaticStore:
		insts = append(insts, generateStaticStore(v))
	case ssa.OpLocalAddr:
		insts = append(insts, generateLocalAddr(v))
	case ssa.OpFieldAddr:
		insts = append(insts, generateFieldAddr(v))
	case ssa.OpStore:
		insts = append(insts, generateStore(v))
	case ssa.OpLoad:
		insts = append(insts, generateLoad(v))
	case ssa.OpX64ADD:
		insts = append(insts, generateBop(v, addOp(v.Type.Size()))...)
	case ssa.OpX64SUB:
		insts = append(insts, generateBop(v, subOp(v.Type.Size()))...)
	case ssa.OpX64IMUL:
		insts = append(insts, generateBop(v, mulOp(v.Type.Size()))...)
	case ssa.OpX64IDIV:
		insts = append(insts, generateDiv(v)...)
	case ssa.OpEqual:
		insts = append(insts, generateCompare(v, "sete")...)
	case ssa.OpNotEqual:
		insts = append(insts, generateCompare(v, "setne")...)
	case ssa.OpLess:
		insts = append(insts, generateCompare(v, "setl")...)
	case ssa.OpLessEqual:
		insts = append(insts, generateCompare(v, "setle")...)
	case ssa.OpGreater:
		insts = append(insts, generateCompare(v, "setg")...)
	case ssa.OpGreaterEqual:
		insts = append(insts, generateCompare(v, "setge")...)
	case ssa.OpX64NEG:
		insts = append(insts, generateNegate(v)...)
	case ssa.OpX64XORconst:
		insts = append(insts, generateNot(v)...)
	case ssa.OpCopy:
		insts = append(insts, generateCopy(v))
	case ssa.OpStaticCall:
		insts = append(insts, generateCall(v))
	case ssa.OpClosureCall:
		insts = append(insts, generateClosureCall(v))
	case ssa.OpLabelAddr:
		insts = append(insts, generateLabelAddr(v))
	case ssa.OpParam, ssa.OpResult, ssa.OpCallResult, ssa.OpClosurePtr, ssa.OpUnit, ssa.OpPhi:
		// placeholders values
	default:
		return nil, fmt.Errorf("no code generator for op %s", v.Op)
	}

	return insts, nil
}

func generateLocalAddr(v *ssa.Value) Inst {
	return Inst{
		Op:   "leaq", // always use the quadword version of this
		Src1: slotArg(v.Slot(), v.Offset),
		Dest: toArg(v),
	}
}

func generateFieldAddr(v *ssa.Value) Inst {
	return Inst{
		Op:   "leaq", // always use the quadword version of this
		Src1: indirect(v.Args[0], v.Offset),
		Dest: toArg(v),
	}
}

func generateStore(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(v.Args[0]),
		Dest: indirect(v.Args[1], v.Offset),
	}
}

func generateLoad(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: indirect(v.Args[0], v.Offset),
		Dest: toArg(v),
	}
}

func generateCall(v *ssa.Value) Inst {
	return Inst{
		Op:   "call",
		Dest: text(funcLabel(v.Callee())),
	}
}

// generateClosureCall jumps to the code pointer the closure carries alongside its environment.
func generateClosureCall(v *ssa.Value) Inst {
	return Inst{
		Op:   "call",
		Dest: indirectTarget(v.Args[0]),
	}
}

// generateLabelAddr materializes a function's address as a value.
func generateLabelAddr(v *ssa.Value) Inst {
	return Inst{
		Op:   "leaq", // always use the quadword version of this
		Src1: ripRelative(funcLabel(v.Callee())),
		Dest: toArg(v),
	}
}

// register64 is the full 8-byte operand for a physical register.
func register64(r target.Register) Arg {
	return Arg{Kind: KRegister, Reg: r, Value: 8}
}

func generateCopy(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(v.Args[0]),
		Dest: toArg(v),
	}
}

func generateNegate(v *ssa.Value) []Inst {
	return []Inst{
		{
			Op:   movOp(v.Type.Size()),
			Src1: toArg(v.Args[0]),
			Dest: toArg(v),
		},
		{
			Op:   negOp(v.Type.Size()),
			Dest: toArg(v),
		},
	}
}

// generateNot xors its operand with the constant lowering left in v
func generateNot(v *ssa.Value) []Inst {
	return []Inst{
		{
			Op:   movOp(v.Type.Size()),
			Src1: toArg(v.Args[0]),
			Dest: toArg(v),
		},
		{
			Op:   xorOp(v.Type.Size()),
			Src1: immediate(v.Value.(int32)),
			Dest: toArg(v),
		},
	}
}

// generateCompare compares v's operands and writes the flag setOp reads
func generateCompare(v *ssa.Value, setOp string) []Inst {
	return []Inst{
		{
			// borrow the cmp op's width from operands
			Op:   cmpOp(v.Args[0].Type.Size()),
			Src1: toArg(v.Args[1]),
			Dest: toArg(v.Args[0]),
		},
		{
			Op:   setOp,
			Dest: toArg(v),
		},
	}
}

func generateBop(v *ssa.Value, op string) []Inst {
	return []Inst{
		{
			Op:   movOp(v.Type.Size()),
			Src1: toArg(v.Args[0]),
			Dest: toArg(v),
		},
		{
			Op:   op,
			Src1: toArg(v.Args[1]),
			Dest: toArg(v),
		},
	}
}

// generateDiv sign-extends the dividend into rdx before idiv divides rdx:rax
func generateDiv(v *ssa.Value) []Inst {
	size := v.Type.Size()
	eax := Arg{Kind: KRegister, Reg: target.RegA, Value: size}
	return []Inst{
		{Op: cdqOp(size)},
		{Op: idivOp(size), Dest: toArg(v.Args[1])},
		{Op: movOp(size), Src1: eax, Dest: toArg(v)},
	}
}

func generateLiteral(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: literalImmediate(v),
		Dest: toArg(v),
	}
}

// literalImmediate is the immediate operand holding a literal's value
func literalImmediate(v *ssa.Value) Arg {
	if b, ok := v.Value.(bool); ok {
		if b {
			return immediate(1)
		}
		return immediate(0)
	}
	return immediate(v.Value.(int32))
}

func generateStaticLoad(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: slotArg(v.Slot(), v.Offset),
		Dest: toArg(v),
	}
}

func generateStaticStore(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(v.Args[0]),
		Dest: slotArg(v.Slot(), v.Offset),
	}
}

// slotArg is the memory operand addressing a slot in the current frame.
func slotArg(s *ssa.Slot, offset int) Arg {
	return Arg{
		Kind:  KMemory,
		Reg:   s.Loc.Reg,
		Value: s.Loc.Offset + offset,
	}
}

func indirect(ptr *ssa.Value, offset int) Arg {
	return Arg{
		Kind:  KMemory,
		Reg:   ptr.Loc.Reg,
		Value: offset,
	}
}

func toArg(v *ssa.Value) Arg {
	return locArg(v.Loc, v.Type.Size())
}

// locArg addresses a value of the given size held at loc
func locArg(loc ssa.Location, size int) Arg {
	switch loc.Kind {
	case ssa.LocRegister:
		return Arg{
			Kind:  KRegister,
			Reg:   loc.Reg,
			Value: size,
		}
	case ssa.LocMemory:
		return Arg{
			Kind:  KMemory,
			Reg:   loc.Reg,
			Value: loc.Offset,
		}
	}
	return Arg{}
}

func immediate(v int32) Arg {
	return Arg{Kind: KImmediate, Value: v}
}

func label(l string) Inst {
	return Inst{Op: l + ":"}
}

func movOp(size int) string {
	var op string
	switch size {
	case 1:
		op = "movb"
	case 2:
		op = "movw"
	case 4:
		op = "movl"
	default:
		op = "movq"
	}
	return op
}

func addOp(size int) string {
	switch size {
	case 1:
		return "addb"
	case 2:
		return "addw"
	case 4:
		return "addl"
	default:
		return "addq"
	}
}

func subOp(size int) string {
	switch size {
	case 1:
		return "subb"
	case 2:
		return "subw"
	case 4:
		return "subl"
	default:
		return "subq"
	}
}

func negOp(size int) string {
	switch size {
	case 1:
		return "negb"
	case 2:
		return "negw"
	case 4:
		return "negl"
	default:
		return "negq"
	}
}

func cmpOp(size int) string {
	switch size {
	case 1:
		return "cmpb"
	case 2:
		return "cmpw"
	case 4:
		return "cmpl"
	default:
		return "cmpq"
	}
}

func xorOp(size int) string {
	switch size {
	case 1:
		return "xorb"
	case 2:
		return "xorw"
	case 4:
		return "xorl"
	default:
		return "xorq"
	}
}

func cdqOp(size int) string {
	switch size {
	case 4:
		return "cdq"
	default:
		return "cqo"
	}
}

func idivOp(size int) string {
	switch size {
	case 1:
		return "idivb"
	case 2:
		return "idivw"
	case 4:
		return "idivl"
	default:
		return "idivq"
	}
}

func mulOp(size int) string {
	switch size {
	case 1:
		return "imulb"
	case 2:
		return "imulw"
	case 4:
		return "imull"
	default:
		return "imulq"
	}
}

func text(v string) Arg {
	return Arg{Kind: KText, Value: v}
}

// ripRelative addresses a label from the instruction pointer
func ripRelative(label string) Arg {
	return Arg{Kind: KRipRelative, Value: label}
}

// indirectTarget is the call target held in v's register.
func indirectTarget(v *ssa.Value) Arg {
	return Arg{Kind: KIndirect, Reg: v.Loc.Reg}
}

func symbol(name string) string {
	return symbolPrefix() + name
}

func funcLabel(f *ssa.Func) string {
	return symbol(f.Name())
}

// blockLabel names a block, scoped to its function since block ids restart in each one
func blockLabel(f *ssa.Func, b *ssa.Block) string {
	return fmt.Sprintf("%s%s.%d", localPrefix(), f.Name(), b.Id)
}

func epilogueLabel(f *ssa.Func) string {
	return fmt.Sprintf("%s%s.ret", localPrefix(), f.Name())
}
