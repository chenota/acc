package ssa

import (
	"errors"
	"maps"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/register"
)

type liveInterval struct {
	Value *Value
	Start int
	End   int
}

type regInterval struct {
	Reg   register.Register
	Start int
	End   int
}

// regalloc colors SSA values with physical registers
func regalloc(f *Func) error {
	liveIntervals := computeLiveIntervals(f)
	regIntervals := computeRegIntervals(liveIntervals)

	c := newColorer(regIntervals)

	for _, iv := range liveIntervals {
		c.expire(iv.Start)

		// skip values already given a home - a register, or a stack slot fixed by the ABI
		if iv.Value.Loc.Kind != LocNone || !iv.Value.NeedsRegister() {
			continue
		}

		newReg, err := c.take(iv)
		if err != nil {
			return err
		}

		iv.Value.Loc = NewReg(newReg)
		c.hold(iv)
	}

	return nil
}

type colorer struct {
	free      register.Mask
	active    []*liveInterval
	preColors []*regInterval
}

func newColorer(regs []*regInterval) *colorer {
	return &colorer{
		free:      register.Allocatable,
		preColors: regs,
	}
}

// expire frees the registers of intervals ending at or before cutoff
func (c *colorer) expire(cutoff int) {
	var kept []*liveInterval
	for _, iv := range c.active {
		if iv.End <= cutoff {
			c.free = c.free.Include(iv.Value.Loc.Reg)
		} else {
			kept = append(kept, iv)
		}
	}
	c.active = kept
}

func (c *colorer) take(iv *liveInterval) (register.Register, error) {
	free := c.free

	// exclude pre-colored registers that overlap with iv
	for _, rv := range c.preColors {
		if overlap(rv.Start, rv.End, iv.Start, iv.End) {
			free = free.Remove(rv.Reg)
		}
	}

	// first try to pick the hinted-at register
	for h := range iv.Value.Hints() {
		if free.Contains(h) {
			c.free = c.free.Remove(h)
			return h, nil
		}
	}

	// prefer caller-saved registers
	pick := free & register.CallerSaved
	if pick.Count() == 0 {
		pick = free
	}

	// last resort pick a random available register
	reg, ok := pick.One()
	if !ok {
		return 0, errors.New("regalloc: out of registers something has gone very wrong :D")
	}
	c.free = c.free.Remove(reg)
	return reg, nil
}

func (c *colorer) hold(iv *liveInterval) {
	c.active = append(c.active, iv)
}

func computeLiveIntervals(f *Func) []*liveInterval {
	intervals := make(map[int]*liveInterval)

	// touch records that v is live at a certain tick
	touch := func(v *Value, tick int) {
		if iv, ok := intervals[v.Id]; ok {
			iv.Start = min(iv.Start, tick)
			iv.End = max(iv.End, tick)
			return
		}
		intervals[v.Id] = &liveInterval{Value: v, Start: tick, End: tick}
	}

	// every value is live at its definition and at each point it is used
	for tick, v := range iterutil.Enumerate(f.OrderedValues()) {
		touch(v, tick)
		for _, arg := range v.Args {
			touch(arg, tick)
		}

		// ensure the right operand of a bop is live alongside the result so they don't get mapped to the same place
		if v.IsBinaryOp() {
			touch(v.Args[1], tick+1)
		}
	}

	// extend a block's control value to live though the block's entire lifecycle
	for _, b := range f.Blocks {
		if b.Control == nil || len(b.Values) == 0 {
			continue
		}
		last := b.Values[len(b.Values)-1]
		touch(b.Control, intervals[last.Id].End+1)
	}

	// order by start tick ascending
	return slices.SortedFunc(maps.Values(intervals), func(a, b *liveInterval) int {
		return a.Start - b.Start
	})
}

func computeRegIntervals(timeline []*liveInterval) []*regInterval {
	var intervals []*regInterval
	for _, iv := range timeline {
		// a precolored value occupies its register for its whole live range
		if iv.Value.Loc.Kind == LocRegister {
			intervals = append(intervals, &regInterval{
				Reg:   iv.Value.Loc.Reg,
				Start: iv.Start,
				End:   iv.End,
			})
		}
		// route live values around a clobber
		for reg := range iv.Value.Clobbers().All() {
			intervals = append(intervals, &regInterval{
				Reg:   reg,
				Start: iv.Start,
				End:   iv.Start + 1,
			})
		}
	}
	return intervals
}

func overlap(start1 int, end1 int, start2 int, end2 int) bool {
	return start1 < end2 && start2 < end1
}
