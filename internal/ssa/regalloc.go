package ssa

import (
	"errors"
	"slices"

	"github.com/chenota/acc/internal/register"
)

type interval struct {
	Value *Value
	Start int
	End   int
}

// regalloc colors SSA values with physical registers
func regalloc(f *Func) error {
	intervals := computeLiveIntervals(f)

	c := newColorer()

	for _, iv := range intervals {
		c.expire(iv.Start)

		// skip precolored values
		if iv.Value.Loc.Kind == LocRegister || !needsRegister(iv.Value) {
			continue
		}

		newReg, err := c.take()
		if err != nil {
			return err
		}

		iv.Value.Loc = NewReg(newReg)
		c.hold(iv)
	}

	return nil
}

type colorer struct {
	free   register.Mask
	active []*interval
}

func newColorer() *colorer {
	return &colorer{
		free: register.Allocatable,
	}
}

// expire frees the registers of intervals ending at or before cutoff
func (c *colorer) expire(cutoff int) {
	var kept []*interval
	for _, iv := range c.active {
		if iv.End <= cutoff {
			c.free = c.free.Include(iv.Value.Loc.Reg)
		} else {
			kept = append(kept, iv)
		}
	}
	c.active = kept
}

func (c *colorer) take() (register.Register, error) {
	reg, ok := c.free.One()
	if !ok {
		return 0, errors.New("regalloc: out of registers something has gone very wrong lol")
	}
	c.free = c.free.Remove(reg)
	return reg, nil
}

func (c *colorer) hold(iv *interval) {
	c.active = append(c.active, iv)
}

func computeLiveIntervals(f *Func) []*interval {
	timeline := f.OrderedValues()
	intervals := make(map[int]*interval)

	// walk backwards through timeline to deal with loop shenanigans
	for tick := len(timeline) - 1; tick >= 0; tick-- {
		v := timeline[tick]

		if inter, exists := intervals[v.Id]; exists {
			inter.Start = tick
		} else {
			intervals[v.Id] = &interval{Value: v, Start: tick, End: tick}
		}

		for _, arg := range v.Args {
			if _, exists := intervals[arg.Id]; !exists {
				intervals[arg.Id] = &interval{
					Value: arg,
					End:   tick,
				}
			}
		}
	}

	var sortedIntervals []*interval
	for _, interval := range intervals {
		sortedIntervals = append(sortedIntervals, interval)
	}

	// sort intervals by start tick ascending
	slices.SortFunc(sortedIntervals, func(a, b *interval) int {
		return a.Start - b.Start
	})

	return sortedIntervals
}
