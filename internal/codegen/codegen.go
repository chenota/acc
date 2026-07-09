package codegen

import (
	"errors"
	"strconv"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/ssa"
)

var (
	basePointer  = Arg{Kind: KRegister, Reg: register.RegBP, Value: 8}
	stackPointer = Arg{Kind: KRegister, Reg: register.RegSP, Value: 8}
	rax          = Arg{Kind: KRegister, Reg: register.RegA, Value: 8}
	rdi          = Arg{Kind: KRegister, Reg: register.RegDI, Value: 8}
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

	insts = append(insts, callAndExit("_start", mainFunc)...)

	for _, f := range program {
		insts = append(insts, generateFunction(f)...)
	}

	return insts, nil
}

func callAndExit(wrapperLabel string, main *ssa.Func) []Inst {
	return []Inst{
		{
			Op:   ".globl",
			Dest: text(wrapperLabel),
		},
		label(wrapperLabel),
		{
			Op:   "call",
			Dest: text(main.Label()),
		},
		{
			Op:   "movq",
			Src1: rax,
			Dest: rdi,
		},
		{
			Op:   "movq",
			Src1: immediate(60),
			Dest: rax,
		},
		{
			Op: "syscall",
		},
	}
}

func generateFunction(f *ssa.Func) []Inst {
	var insts []Inst

	insts = append(insts,
		Inst{
			Op:   ".globl",
			Dest: text(f.Label()),
		},
		label(f.Label()),
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
	if f.FrameSize() > 0 {
		insts = append(insts, Inst{
			Op:   "subq",
			Src1: immediate(int32(f.FrameSize())),
			Dest: stackPointer,
		})
	}

	for b := range f.OrderedBlocks() {
		insts = append(insts, generateBlock(b)...)
	}

	if f.FrameSize() > 0 {
		insts = append(insts, Inst{
			Op:   "addq",
			Src1: immediate(int32(f.FrameSize())),
			Dest: stackPointer,
		})
	}

	insts = append(insts, Inst{
		Op:   "popq",
		Dest: basePointer,
	})

	insts = append(insts, Inst{Op: "ret"})

	return insts
}

func generateBlock(b *ssa.Block) []Inst {
	var insts []Inst

	// only need a label if something is going to jump to this block
	if len(b.Predecessors) > 0 {
		insts = append(insts, label(blockLabel(b)))
	}

	for _, v := range b.Values {
		insts = append(insts, generateValue(v)...)
	}

	return insts
}

func generateValue(v *ssa.Value) []Inst {
	var insts []Inst

	switch v.Op {
	case ssa.OpLiteral:
		insts = append(insts, generateConstInt(v))
	case ssa.OpLoad:
		insts = append(insts, generateLoad(v))
	case ssa.OpStore:
		insts = append(insts, generateStore(v))
	case ssa.OpAdd:
		insts = append(insts, generateBop(v, addOp(v.Type.Size()))...)
	case ssa.OpSubtract:
		insts = append(insts, generateBop(v, subOp(v.Type.Size()))...)
	case ssa.OpMultiply:
		insts = append(insts, generateBop(v, mulOp(v.Type.Size()))...)
	case ssa.OpDivide:
		insts = append(insts, generateDiv(v)...)
	case ssa.OpNegate:
		insts = append(insts, generateNegate(v)...)
	case ssa.OpCopy:
		insts = append(insts, generateCopy(v))
	case ssa.OpSignExtend:
		insts = append(insts, generateSignExtend(v))
	}

	return insts
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

func generateDiv(v *ssa.Value) []Inst {
	size := v.Type.Size()
	eax := Arg{Kind: KRegister, Reg: register.RegA, Value: size}
	return []Inst{
		{Op: idivOp(size), Dest: toArg(v.Args[1])},
		{Op: movOp(size), Src1: eax, Dest: toArg(v)},
	}
}

func generateSignExtend(v *ssa.Value) Inst {
	return Inst{Op: cdqOp(v.Type.Size())}
}

func generateConstInt(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: immediate(v.Value.(int32)),
		Dest: toArg(v),
	}
}

func generateLoad(v *ssa.Value) Inst {
	alloca := v.Args[0]

	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(alloca),
		Dest: toArg(v),
	}
}

func generateStore(v *ssa.Value) Inst {
	src := v.Args[0]
	alloca := v.Args[1]

	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(src),
		Dest: toArg(alloca),
	}
}

func blockLabel(b *ssa.Block) string {
	return "_block" + strconv.Itoa(b.Id)
}

func toArg(v *ssa.Value) Arg {
	switch v.Loc.Kind {
	case ssa.LocRegister:
		return Arg{
			Kind:  KRegister,
			Reg:   v.Loc.Reg,
			Value: v.Type.Size(),
		}
	case ssa.LocStack:
		return Arg{
			Kind:  KStack,
			Value: v.Loc.Offset,
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
