package codegen

import (
	"errors"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/ssa"
)

var (
	basePointer  = Arg{Kind: KRegister, Reg: register.RegBP, Value: 8}
	stackPointer = Arg{Kind: KRegister, Reg: register.RegSP, Value: 8}
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
		insts = append(insts, generateFunction(f)...)
	}

	// remove all redundant mov instructions
	insts = movElim(insts)

	return insts, nil
}

func generateFunction(f *ssa.Func) []Inst {
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
	saved := (f.UsedRegisters() & register.CalleeSaved).All()
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

	for b := range f.OrderedBlocks() {
		insts = append(insts, generateBlock(b)...)
	}

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

	return insts
}

func generateBlock(b *ssa.Block) []Inst {
	var insts []Inst

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
	case ssa.OpStaticLoad:
		insts = append(insts, generateStaticLoad(v))
	case ssa.OpStaticStore:
		insts = append(insts, generateStaticStore(v))
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
	case ssa.OpStaticCall:
		insts = append(insts, generateCall(v))
	}

	return insts
}

func generateCall(v *ssa.Value) Inst {
	return Inst{
		Op:   "call",
		Dest: text(funcLabel(v.Callee())),
	}
}

// register64 is the full 8-byte operand for a physical register.
func register64(r register.Register) Arg {
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

func generateStaticLoad(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: slotArg(v.Slot()),
		Dest: toArg(v),
	}
}

func generateStaticStore(v *ssa.Value) Inst {
	return Inst{
		Op:   movOp(v.Type.Size()),
		Src1: toArg(v.Args[0]),
		Dest: slotArg(v.Slot()),
	}
}

// slotArg is the memory operand addressing a slot in the current frame.
func slotArg(s *ssa.Slot) Arg {
	return Arg{
		Kind:  KMemory,
		Reg:   s.Loc.Reg,
		Value: s.Loc.Offset,
	}
}

func toArg(v *ssa.Value) Arg {
	switch v.Loc.Kind {
	case ssa.LocRegister:
		return Arg{
			Kind:  KRegister,
			Reg:   v.Loc.Reg,
			Value: v.Type.Size(),
		}
	case ssa.LocMemory:
		return Arg{
			Kind:  KMemory,
			Reg:   v.Loc.Reg,
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

// symbol maps a source-level name to its assembly symbol, applying the target's
// C symbol convention (see symbolPrefix).
func symbol(name string) string {
	return symbolPrefix + name
}

// funcLabel is the assembly symbol for a function.
func funcLabel(f *ssa.Func) string {
	return symbol(f.Name())
}
