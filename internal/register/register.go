package register

import "math/bits"

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

	numRegisters
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

// Complement returns the registers not in m
func (m Mask) Complement() Mask {
	return (^m) & ((1 << numRegisters) - 1)
}

// Count returns the count of registers in m
func (m Mask) Count() int {
	return bits.OnesCount64(uint64(m))
}

var (
	CallerSaved  = NewMask(RegA, RegC, RegD, RegSI, RegDI, Reg8, Reg9, Reg10, Reg11)
	CalleeSaved  = NewMask(RegB, Reg12, Reg13, Reg14, Reg15)
	Reserved     = NewMask(RegSP, RegBP)
	ReturnTarget = RegA
)
