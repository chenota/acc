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

	for _, b := range f.OrderedBlocks() {
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
		insts = append(insts, generateConstInt32(v))
	case ssa.OpLoad:
		insts = append(insts, generateLoad(v))
	case ssa.OpStore:
		insts = append(insts, generateStore(v))
	}

	return insts
}

func generateConstInt32(v *ssa.Value) Inst {
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

func text(v string) Arg {
	return Arg{Kind: KText, Value: v}
}
