package codegen

import (
	"strconv"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/ssa"
)

var (
	basePointer  = Arg{Kind: KRegister, Reg: register.RegBP, Value: 64}
	stackPointer = Arg{Kind: KRegister, Reg: register.RegSP, Value: 64}
)

func GenerateProgram(program []*ssa.Func) []Inst {
	var insts []Inst

	for _, f := range program {
		insts = append(insts, generateFunction(f)...)
	}

	return insts
}

func generateFunction(f *ssa.Func) []Inst {
	var insts []Inst

	insts = append(insts, label(f.Name))
	insts = append(insts, Inst{
		Op:   "pushq",
		Dest: basePointer,
	})
	insts = append(insts, Inst{
		Op:   "movq",
		Src1: stackPointer,
		Dest: basePointer,
	})
	if f.StackSize() > 0 {
		insts = append(insts, Inst{
			Op:   "subq",
			Src1: immediate(f.StackSize()),
			Dest: stackPointer,
		})
	}

	for _, b := range f.OrderedBlocks() {
		insts = append(insts, generateBlock(b)...)
	}

	if f.StackSize() > 0 {
		insts = append(insts, Inst{
			Op:   "addq",
			Src1: immediate(f.StackSize()),
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

	for _, v := range b.OrderedValues() {
		insts = append(insts, generateValue(v)...)
	}

	return insts
}

func generateValue(v *ssa.Value) []Inst {
	var insts []Inst

	switch v.Op {
	case ssa.OpConstInt32:
		insts = append(insts, generateConstInt32(v))
	case ssa.OpLoadReg:
		insts = append(insts, generateLoad(v))
	case ssa.OpStoreReg:
		insts = append(insts, generateStore(v))
	}

	return insts
}

func generateConstInt32(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(32),
		Src1: immediate(v.AuxInt),
		Dest: toArg(v),
	}
}

func generateLoad(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: stack(v.AuxInt),
		Dest: toArg(v),
	}
}

func generateStore(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(v.Args[0]),
		Dest: toArg(v),
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
			Value: int64(v.Type.Size()),
		}
	case ssa.LocStack:
		return Arg{
			Kind:  KStack,
			Reg:   v.Loc.Reg,
			Value: int64(v.Loc.Slot),
		}
	}
	return Arg{}
}

func immediate(v int64) Arg {
	return Arg{Kind: KImmediate, Value: v}
}

func label(l string) Inst {
	return Inst{Op: l + ":"}
}

func stack(slot int64) Arg {
	return Arg{Kind: KStack, Value: slot}
}

func movOp(size int) string {
	var op string
	switch size {
	case 8:
		op = "movb"
	case 16:
		op = "movw"
	case 32:
		op = "movl"
	default:
		op = "movq"
	}
	return op
}
