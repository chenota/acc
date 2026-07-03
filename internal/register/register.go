package register

type Register int

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

// Mask returns a mask containing just the Register
func (r Register) Mask() Mask {
	return 1 << r
}

type Mask uint64

// NewMask creates a new mask containing a set of registers
func NewMask(regs ...Register) Mask {
	var m Mask
	for _, r := range regs {
		m |= r.Mask()
	}
	return m
}

var (
	CallerSaved  = NewMask(RegA, RegC, RegD, RegSI, RegDI, Reg8, Reg9, Reg10, Reg11)
	CalleeSaved  = NewMask(RegB, Reg12, Reg13, Reg14, Reg15)
	Reserved     = NewMask(RegSP, RegBP)
	ReturnTarget = RegA
)
