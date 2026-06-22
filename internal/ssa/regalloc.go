package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
)

type interval struct {
	Value *Value
	Start int
	End   int
}

func regalloc(f *Func, registers registerGroup) {
	timeline := f.values()
	intervals := computeLiveIntervals(timeline)

	r := newRegisterAllocater(registers)

	r.prepareReturns(f)

	for _, curr := range intervals {
		r.expireOldIntervals(curr.Start)

		// current is pre-filled we must spill the existing value if something is using it
		if curr.Value.Loc.Kind == LocRegister {
			r.processPreAllocatedInterval(f, curr)
			continue
		}

		// is there a free register we can give this value?
		if reg, ok := r.freeRegister(); ok {
			r.assignRegister(curr, reg)
		}

		// last resort: spill a current active or self
		r.spillOrEvict(f, curr)
	}

	// inject load and store operations given values that were evicted to the stack
	r.injectLoadsAndStores(f)
}

type registerAllocater struct {
	working      map[register.Register]bool
	scratch      []register.Register
	returnTarget register.Register

	active []*interval

	spillMap map[int]*Value
}

func (r *registerAllocater) prepareReturns(f *Func) {
	for _, block := range f.Blocks {
		if block.Kind == BlockRet {
			block.Control.Loc = NewReg(r.returnTarget)
		}
	}
}

func newRegisterAllocater(registers registerGroup) *registerAllocater {
	working := make(map[register.Register]bool)
	for _, r := range registers.working {
		working[r] = true
	}

	return &registerAllocater{
		working:      working,
		scratch:      registers.scratch,
		returnTarget: registers.returnTarget,
		spillMap:     make(map[int]*Value),
	}
}

func (r *registerAllocater) spillOrEvict(f *Func, i *interval) {
	spill := r.latestActive()

	// target interval takes too much time; directly spill target
	if i.End > spill.End {
		r.spillValue(f, i)
		return
	}

	r.evictInterval(f, spill)
	r.addActive(i)
}

// expireOldIntervals moves all registers taken by expired values back into the free pool
func (r *registerAllocater) expireOldIntervals(cutoff int) {
	for tick, interval := range r.active {
		if interval.End >= cutoff {
			r.active = r.active[tick:]
			return
		}
		r.working[interval.Value.Loc.Reg] = true
	}
	r.active = nil
}

// freeRegister returns the first free register in the file if any exist
func (r *registerAllocater) freeRegister() (register.Register, bool) {
	for reg, isFree := range r.working {
		if isFree {
			return reg, true
		}
	}

	return 0, false
}

func (r *registerAllocater) processPreAllocatedInterval(f *Func, i *interval) {
	targetRegister := i.Value.Loc.Reg

	// see if any currently active intervals hold that register
	var theif *interval
	for _, active := range r.active {
		if active.Value.Loc.Kind == LocRegister && active.Value.Loc.Reg == targetRegister {
			theif = active
		}
	}

	// nothing has that register; assign it and mark it as taken
	if theif == nil {
		r.assignRegister(i, targetRegister)
		return
	}

	r.evictInterval(f, theif)
	r.addActive(i)
}

func (r *registerAllocater) injectLoadsAndStores(f *Func) {
	for _, block := range f.Blocks {
		for _, v := range block.Values {
			scratchCount := 0

			// uses - load before instruction
			for idx, arg := range v.Args {
				if alloca, ok := r.spillMap[arg.Id]; ok {
					// create a load instruction using a dedicated scratch register
					load := f.newValue(OpLoad, arg.Type, block)
					load.Args = []*Value{alloca}
					load.Loc = NewReg(r.scratchRegister(scratchCount))

					// rewrite the instruction's argument to point to the result of our load
					v.Args[idx] = load

					scratchCount += 1
				}
			}

			// defs - spill after instruction
			if alloca, ok := r.spillMap[v.Id]; ok {
				store := f.newValue(OpStore, v.Type, block)
				store.Args = []*Value{v, alloca}

				v.Loc = NewReg(r.scratchRegister(scratchCount))

				scratchCount += 1
			}
		}
	}
}

func (r *registerAllocater) assignRegister(i *interval, reg register.Register) {
	r.working[reg] = false
	i.Value.Loc = NewReg(reg)
	r.addActive(i)
}

func (r *registerAllocater) evictInterval(f *Func, i *interval) {
	// see if there's a different free register we can give this value
	if free, ok := r.freeRegister(); ok {
		r.working[free] = false
		i.Value.Loc.Reg = free
		return
	}

	// this interval must be spilled
	r.removeActive(i)
	r.spillValue(f, i)
}

func (r *registerAllocater) spillValue(f *Func, i *interval) {
	alloca := f.newValue(OpAlloca, i.Value.Type, i.Value.Block)
	r.spillMap[i.Value.Id] = alloca
}

func (r *registerAllocater) latestActive() *interval {
	return r.active[len(r.active)-1]
}

func (r *registerAllocater) addActive(i *interval) {
	r.active = append(r.active, i)
	slices.SortFunc(r.active, func(a, b *interval) int { return a.End - b.End })
}

func (r *registerAllocater) removeActive(i *interval) {
	r.active = slices.DeleteFunc(r.active, func(a *interval) bool { return a == i })
}

func (r *registerAllocater) scratchRegister(idx int) register.Register {
	return r.scratch[idx%len(r.scratch)]
}

func computeLiveIntervals(timeline []*Value) []*interval {
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
