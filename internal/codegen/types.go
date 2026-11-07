package codegen

import "github.com/chenota/acc/internal/target"

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
	KIndirect    // a jump or call target held in a register
)

type Arg struct {
	Kind ArgKind

	Reg target.Register

	Value any
}
