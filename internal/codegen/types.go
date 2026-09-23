package codegen

import "github.com/chenota/acc/internal/register"

type Inst struct {
	Op   string
	Dest Arg
	Src1 Arg
	Src2 Arg
}

type ArgKind int

const (
	KUndefined ArgKind = iota
	KRegister
	KImmediate
	KMemory
	KText
	KRipRelative // a label addressed relative to the instruction pointer
	KIndirect    // the address held in a register, as a jump or call target
)

type Arg struct {
	Kind ArgKind

	Reg register.Register

	Value any
}
