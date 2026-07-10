package ssa

import (
	"errors"
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

		// skip precolored values
		if iv.Value.Loc.Kind == LocRegister || !iv.Value.NeedsRegister() {
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
	reg, ok := free.One()
	if !ok {
		return 0, errors.New("regalloc: out of registers something has gone very wrong lol")
	}
	c.free = c.free.Remove(reg)
	return reg, nil
}

func (c *colorer) hold(iv *liveInterval) {
	c.active = append(c.active, iv)
}

func computeLiveIntervals(f *Func) []*liveInterval {
	intervals := make(map[int]*liveInterval)

	// walk backwards through timeline to deal with loop shenanigans
	for tick, v := range iterutil.Reverse2(iterutil.Enumerate(f.OrderedValues())) {
		if inter, exists := intervals[v.Id]; exists {
			inter.Start = tick
		} else {
			intervals[v.Id] = &liveInterval{Value: v, Start: tick, End: tick}
		}

		for _, arg := range v.Args {
			if _, exists := intervals[arg.Id]; !exists {
				intervals[arg.Id] = &liveInterval{
					Value: arg,
					End:   tick,
				}
			}
		}
	}

	var sortedIntervals []*liveInterval
	for _, interval := range intervals {
		sortedIntervals = append(sortedIntervals, interval)
	}

	// sort intervals by start tick ascending
	slices.SortFunc(sortedIntervals, func(a, b *liveInterval) int {
		return a.Start - b.Start
	})

	return sortedIntervals
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
		for _, reg := range iv.Value.Clobbers {
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
