package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/types"
)

type liveInterval struct {
	Value *Value
	Start int
	End   int
}

func regalloc(f *Func) {
	timeline := f.values()
	intervals := computeLiveIntervals(timeline)

	file := &registerFile{
		workingRegisters: []int{0, 1},
		scratchRegisters: []int{2, 3},
	}

	for _, curr := range intervals {
		file.expireOldIntervals(curr)

		if reg, ok := file.free(); ok {
			curr.Value.Loc = NewReg(reg)
			file.addActive(curr)
		} else {
			spill := file.lastActive()
			if curr.End > spill.End {
				curr.Value.Loc = NewStack(f.allocateSpill())
				continue
			}
			curr.Value.Loc = spill.Value.Loc
			injectSpill(f, spill.Value)
			file.setLastActive(curr)
		}
	}

	// inject load instructions for "STACK" values
	for _, val := range timeline {
		loadCount := 0
		for _, arg := range val.Args {
			if arg.Loc.Kind == LocStack {
				injectLoad(f, val, arg, file.scratchRegister(loadCount))
				loadCount++
			}
		}
	}
}

func injectSpill(f *Func, v *Value) {
	spill := f.newValue(OpStoreReg, types.Mem(), v.Block)
	spill.Args = []*Value{v}

	spill.Loc = v.Loc
	v.Loc = NewStack(f.allocateSpill())

	// inject spill value immediately after current value
	for i, blockVal := range v.Block.Values {
		if blockVal.Id == v.Id {
			v.Block.Values = slices.Insert(v.Block.Values, i+1, spill)
			break
		}
	}
}

func injectLoad(f *Func, target *Value, spilledValue *Value, scratchRegister int) {
	load := f.newValue(OpLoadReg, spilledValue.Type, target.Block)
	load.Loc = NewReg(scratchRegister)

	for i, blockVal := range target.Block.Values {
		if blockVal.Id == target.Id {
			target.Block.Values = slices.Insert(target.Block.Values, i, load)
			break
		}
	}

	for idx, arg := range target.Args {
		if arg.Id == spilledValue.Id {
			target.Args[idx] = load
		}
	}
}

func computeLiveIntervals(timeline []*Value) []*liveInterval {
	intervals := make(map[int]*liveInterval)

	// walk backwards through timeline
	for tick := len(timeline) - 1; tick >= 0; tick-- {
		v := timeline[tick]

		if interval, exists := intervals[v.Id]; exists {
			interval.Start = tick
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

	slices.SortFunc(sortedIntervals, func(a, b *liveInterval) int {
		return a.Start - b.Start
	})

	return sortedIntervals
}

type registerFile struct {
	workingRegisters []int
	scratchRegisters []int

	active []*liveInterval
}

// expireOldIntervals moves all registers taken by expired values back into the free pool
func (r *registerFile) expireOldIntervals(i *liveInterval) {
	for tick, interval := range r.active {
		if interval.End >= i.Start {
			r.active = r.active[tick:]
			return
		}
		r.workingRegisters = append(r.workingRegisters, interval.Value.Loc.Reg)
	}
	r.active = nil
}

// free returns the first free register in the file if any exist
func (r *registerFile) free() (int, bool) {
	if len(r.workingRegisters) == 0 {
		return 0, false
	}

	reg := r.workingRegisters[0]
	r.workingRegisters = r.workingRegisters[1:]
	return reg, true
}

func (r *registerFile) lastActive() *liveInterval {
	return r.active[len(r.active)-1]
}

func (r *registerFile) setLastActive(i *liveInterval) {
	r.active[len(r.active)-1] = i
	r.sortActive()
}

func (r *registerFile) addActive(i *liveInterval) {
	r.active = append(r.active, i)
	r.sortActive()
}

func (r *registerFile) sortActive() {
	slices.SortFunc(r.active, func(a, b *liveInterval) int { return a.End - b.End })
}

func (r *registerFile) scratchRegister(i int) int {
	return r.scratchRegisters[i%len(r.scratchRegisters)]
}
