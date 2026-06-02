package codegen

import (
	"fmt"
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
		Args: []Arg{{Kind: KRegister, AuxInt: BasePointer}},
	})
	insts = append(insts, Inst{
		Op: "movq",
		Args: []Arg{
			{Kind: KRegister, AuxInt: StackPointer},
			{Kind: KRegister, AuxInt: BasePointer},
		},
	})
	if f.AllocSize() > 0 {
		insts = append(insts, Inst{
			Op: "subq",
			Args: []Arg{
				{Kind: KImmediate, AuxInt: f.AllocSize()},
				{Kind: KRegister, AuxInt: StackPointer},
			},
		})
	}

	for _, b := range f.OrderedBlocks() {
		insts = append(insts, generateBlock(b)...)
	}

	if f.AllocSize() > 0 {
		insts = append(insts, Inst{
			Op: "addq",
			Args: []Arg{
				{Kind: KImmediate, AuxInt: f.AllocSize()},
				{Kind: KRegister, AuxInt: StackPointer},
			},
		})
	}

	insts = append(insts, Inst{
		Op:   "popq",
		Args: []Arg{{Kind: KRegister, AuxInt: BasePointer}},
	})

	insts = append(insts, Inst{Op: "ret"})

	return insts
}

func generateBlock(b *ssa.Block) []Inst {
	var insts []Inst

	insts = append(insts, labelInst(blockLabel(b)))

	for _, v := range b.OrderedValues() {
		fmt.Println("fuck")
		fmt.Println(v.Op)
		insts = append(insts, generateValue(v)...)
	}

	return insts
}

func generateValue(v *ssa.Value) []Inst {
	var insts []Inst

	switch v.Op {
	case ssa.OpConstInt32:
		insts = append(insts, generateConstInt32(v))
	case ssa.OpCopy:
		insts = append(insts, generateCopy(v))
	case ssa.OpLoadReg:
	case ssa.OpStoreReg:
	}

	return insts
}

func generateCopy(v *ssa.Value) Inst {
	source := toArg(v.Args[0].Loc)
	dest := toArg(v.Loc)

	switch v.Type.Size() {
	case 32:
		return Inst{
			Op:   "movl",
			Args: []Arg{source, dest},
		}
	default:
		return Inst{}
	}
}

func generateConstInt32(v *ssa.Value) Inst {
	dest := toArg(v.Loc)

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
