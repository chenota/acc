package codegen

import (
	"strconv"

	"github.com/chenota/acc/internal/ssa"
)

const BasicBlockPrefix string = "__bb"
const FunctionPrefix string = "_f"

const (
	StackPointer = 6
	BasePointer  = 7
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

	insts = append(insts, labelInst(funcLabel(f)))
	insts = append(insts, Inst{
		Op:   "pushq",
		Args: []Arg{{Kind: KRegister, Size: Size64, AuxInt: BasePointer}},
	})
	insts = append(insts, Inst{
		Op: "movq",
		Args: []Arg{
			{Kind: KRegister, Size: Size64, AuxInt: StackPointer},
			{Kind: KRegister, Size: Size64, AuxInt: BasePointer},
		},
	})
	if f.AllocSize() > 0 {
		insts = append(insts, Inst{
			Op: "subq",
			Args: []Arg{
				{Kind: KImmediate, AuxInt: f.AllocSize()},
				{Kind: KRegister, Size: Size64, AuxInt: StackPointer},
			},
		})
	}

	blocks := f.OrderedBlocks()

	for _, b := range blocks {
		insts = append(insts, generateBlock(b)...)
	}

	if f.AllocSize() > 0 {
		insts = append(insts, Inst{
			Op: "addq",
			Args: []Arg{
				{Kind: KImmediate, AuxInt: f.AllocSize()},
				{Kind: KRegister, Size: Size64, AuxInt: StackPointer},
			},
		})
	}

	insts = append(insts, Inst{
		Op:   "popq",
		Args: []Arg{{Kind: KRegister, Size: Size64, AuxInt: BasePointer}},
	})

	insts = append(insts, Inst{Op: "ret"})

	return insts
}

func generateBlock(b *ssa.Block) []Inst {
	var insts []Inst

	insts = append(insts, labelInst(blockLabel(b)))

	for _, v := range b.Values {
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
	case ssa.OpStoreReg:
	}

	return insts
}

func generateConstInt32(v *ssa.Value) Inst {
	dest := toArg(v.Loc)
	dest.Size = Size32

	return Inst{
		Op:   "movl",
		Args: []Arg{{Kind: KImmediate, AuxInt: v.AuxInt}, dest},
	}
}

func labelInst(label string) Inst {
	return Inst{Op: label + ":"}
}

func blockLabel(b *ssa.Block) string {
	return BasicBlockPrefix + strconv.Itoa(b.Id)
}

func funcLabel(f *ssa.Func) string {
	return FunctionPrefix + f.Name
}

func toArg(l ssa.Location) Arg {
	switch l.Kind {
	case ssa.LocRegister:
		return Arg{
			Kind:   KRegister,
			AuxInt: int64(l.Reg),
		}
	case ssa.LocStack:
		return Arg{
			Kind:   KStack,
			AuxInt: int64(l.Slot * 8),
		}
	}
	return Arg{}
}
