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

type registerGroup struct {
	working      []register.Register
	scratch      []register.Register
	returnTarget register.Register
}

var registerFile = registerGroup{
	working:      []register.Register{register.Reg8, register.Reg9, register.Reg10, register.Reg11, register.Reg12, register.Reg13},
	scratch:      []register.Register{register.Reg14, register.Reg15},
	returnTarget: register.RegA,
}

func regalloc(f *Func) {
	timeline := f.OrderedValues()
	intervals := computeLiveIntervals(timeline)

	r := newRegisterAllocater(registerFile)

	r.prepareReturns(f)
	prepareDivides(f)

	for _, curr := range intervals {
		r.expireOldIntervals(curr.Start)

		// current is pre-filled we must spill the existing value if something is using it
		if curr.Value.Loc.Kind == LocRegister {
			r.processPreAllocatedInterval(f, curr)
			// idivl overwrites %edx so evict any value live there
			if curr.Value.Op == OpDivide {
				r.evictRegister(f, register.RegD)
			}
			continue
		}

		// is there a free register we can give this value?
		if reg, ok := r.freeRegister(); ok {
			r.assignRegister(curr, reg)
			continue
		}

		// last resort: spill a current active or self
		r.spillOrEvict(f, curr)
	}

	// inject load and store operations given values that were evicted to the stack
	r.injectLoadsAndStores(f)
}

func prepareDivides(f *Func) {
	for _, v := range f.OrderedValues() {
		if v.Op == OpDivide {
			v.Loc = NewReg(register.RegA) // divide always goes in register A
		}
	}
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
	theif := r.activeWithRegiser(targetRegister)

	// nothing has that register; assign it and mark it as taken
	if theif == nil {
		r.assignRegister(i, targetRegister)
		return
	}

	r.evictInterval(f, theif)
	r.addActive(i)
}

func (r *registerAllocater) activeWithRegiser(reg register.Register) *interval {
	for _, active := range r.active {
		if active.Value.Loc.Kind == LocRegister && active.Value.Loc.Reg == reg {
			return active
		}
	}

	return nil
}

func (r *registerAllocater) injectLoadsAndStores(f *Func) {
	for _, block := range f.Blocks {
		for _, v := range block.Values {
			scratchCount := 0

			// uses - load before instruction
			for idx, arg := range v.Args {
				if alloca, ok := r.spillMap[arg.Id]; ok {
					// create a load instruction using a dedicated scratch register
					load := f.insertValueBefore(v, OpLoad, arg.Type, block)
					load.Args = []*Value{alloca}
					load.Loc = NewReg(r.scratchRegister(scratchCount))

					// rewrite the instruction's argument to point to the result of our load
					v.Args[idx] = load

					scratchCount += 1
				}
			}

			// defs - spill after instruction
			if alloca, ok := r.spillMap[v.Id]; ok {
				store := f.insertValueAfter(v, OpStore, v.Type, block)
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

func (r *registerAllocater) evictRegister(f *Func, reg register.Register) {
	theif := r.activeWithRegiser(reg)
	if theif != nil {
		r.evictInterval(f, theif)
	}
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
	// construct a new alloca but don't add it to the block's value list so that it never generates instructions
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
