package ssa

import (
	"fmt"

	"github.com/chenota/acc/internal/register"
)

// regalloc colors SSA values with physical registers
func regalloc(f *Func) error {
	// spilling inserted stores and reloads, so the ranges are worked out fresh
	ranges := computeLiveRanges(f)

	// in definition order, everything live alongside a value and defined before it already has its register
	for v := range f.OrderedValues() {
		// skip values already put somewhere
		if v.Loc.Kind != LocNone || !v.NeedsRegister() {
			continue
		}

		reg, ok := pick(v, register.Allocatable.Without(ranges.blockedFor(v)))
		if !ok {
			return fmt.Errorf("regalloc: out of registers for v%d in block %d of %s, something has gone very wrong :D", v.Id, v.Block.Id, f.Name())
		}
		v.Loc = NewReg(reg)
	}

	return nil
}

// pick chooses v's register out of free
func pick(v *Value, free register.Mask) (register.Register, bool) {
	// first try to pick the hinted-at register
	for h := range v.Hints() {
		if free.Contains(h) {
			return h, true
		}
	}

	// prefer caller-saved registers
	if callerSaved := free.Intersect(register.CallerSaved); callerSaved.Count() > 0 {
		return callerSaved.One()
	}

	// last resort pick an arbitrary available register
	return free.One()
}
