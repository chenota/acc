package target

const (
	RegA Register = iota
	RegB
	RegC
	RegD
	RegSI
	RegDI
	RegSP
	RegBP
	Reg8
	Reg9
	Reg10
	Reg11
	Reg12
	Reg13
	Reg14
	Reg15
)

// X64SysV is x64 under the System V calling convention.
var X64SysV = &Target{
	Allocatable: NewMask(RegA, RegB, RegC, RegD, RegSI, RegDI, Reg8, Reg9, Reg10, Reg11, Reg12, Reg13, Reg14, Reg15),
	CallerSaved: NewMask(RegA, RegC, RegD, RegSI, RegDI, Reg8, Reg9, Reg10, Reg11),
	CalleeSaved: NewMask(RegB, Reg12, Reg13, Reg14, Reg15),

	Args:           []Register{RegDI, RegSI, RegD, RegC, Reg8, Reg9},
	Results:        []Register{RegA, RegD},
	ClosureContext: Reg10,

	StackPointer: RegSP,
	FramePointer: RegBP,
}
