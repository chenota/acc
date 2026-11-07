package target

import (
	"iter"
	"math/bits"
	"slices"
)

type Register int

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

// Count returns the count of registers in m
func (m Mask) Count() int {
	return bits.OnesCount64(uint64(m))
}

// One returns a single register from mask m
func (m Mask) One() (Register, bool) {
	if m.Count() == 0 {
		return 0, false
	}

	return Register(bits.TrailingZeros64(uint64(m))), true
}

// All returns all registers from mask m
func (m Mask) All() iter.Seq[Register] {
	var regs []Register
	for m > 0 {
		pos := bits.TrailingZeros64(uint64(m))
		regs = append(regs, Register(pos))

		// clear lowest bit
		m = m & (m - 1)
	}

	return slices.Values(regs)
}

// Include adds a register to the mask
func (m Mask) Include(r Register) Mask {
	return m | r.Mask()
}

// Remove removes a register from the mask
func (m Mask) Remove(r Register) Mask {
	return m & ^r.Mask()
}

// Without removes every register in other from the mask
func (m Mask) Without(other Mask) Mask {
	return m &^ other
}

// Union returns the registers in either mask
func (m Mask) Union(other Mask) Mask {
	return m | other
}

// Intersect returns the registers in both masks
func (m Mask) Intersect(other Mask) Mask {
	return m & other
}

// Contains checks if a register is contained in the mask
func (m Mask) Contains(r Register) bool {
	return m&r.Mask() != 0
}
