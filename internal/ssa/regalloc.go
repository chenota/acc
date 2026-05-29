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
	timeline := generateTimeline(f.Entry)
	intervals := computeLiveIntervals(timeline)

	file := &registerFile{
		workingRegisters: []string{"rax", "rbx"},
		scratchRegisters: []string{"rcx", "rdx"},
	}

	for _, curr := range intervals {
		file.expireOldIntervals(curr)

		if reg := file.free(); reg != "" {
			curr.Value.Register = reg
			file.addActive(curr)
		} else {
			spill := file.lastActive()
			if curr.End > spill.End {
				injectSpill(f, curr.Value)
				continue
			}
			curr.Value.Register = spill.Value.Register
			injectSpill(f, spill.Value)
			file.setLastActive(curr)
		}
	}

	// inject load instructions for "STACK" values
	for _, val := range timeline {
		loadCount := 0
		for _, arg := range val.Args {
			if arg.Register == "STACK" {
				injectLoad(f, val, arg, loadCount)
				loadCount++
			}
		}
	}
}

type registerFile struct {
	workingRegisters []string
	scratchRegisters []string

	active []*liveInterval
}

// takeFree attempts to allocate interval i to a free register. Returns false if there are no free registers.
func (r *registerFile) takeFree(i *liveInterval) bool {
	if len(r.workingRegisters) == 0 {
		return false
	}

	reg := r.workingRegisters[0]
	r.workingRegisters = r.workingRegisters[1:]

	i.Value.Register = reg

	r.active = append(r.active, i)
	r.sortActive()

	return true
}

// expireOldIntervals moves all registers taken by expired values back into the free pool
func (r *registerFile) expireOldIntervals(i *liveInterval) {
	for tick, interval := range r.active {
		if interval.End >= i.Start {
			r.active = r.active[tick:]
			return
		}
		r.workingRegisters = append(r.workingRegisters, interval.Value.Register)
	}
	r.active = nil
}

func (r *registerFile) sortActive() {
	slices.SortFunc(r.active, func(a, b *liveInterval) int { return a.End - b.End })
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

func (r *registerFile) free() string {
	if len(r.workingRegisters) == 0 {
		return ""
	}

	reg := r.workingRegisters[0]
	r.workingRegisters = r.workingRegisters[1:]
	return reg
}

func injectSpill(f *Func, v *Value) {
	spill := f.newValue(OpStoreReg, types.Mem(), v.Block)
	spill.Args = []*Value{v}
	spill.AuxInt = int64(f.allocateSpill())

	spill.Register = v.Register
	v.Register = "STACK"

	// inject spill value immediately after current value
	for i, blockVal := range v.Block.Values {
		if blockVal.Id == v.Id {
			v.Block.Values = slices.Insert(v.Block.Values, i+1, spill)
			break
		}
	}
}

func injectLoad(f *Func, target *Value, spilledValue *Value, scratchIndex int) {
	load := f.newValue(OpLoadReg, spilledValue.Type, target.Block)
	load.AuxInt = spilledValue.AuxInt // copy the stack slot index

	scratchRegs := []string{"rcx", "rdx"}
	load.Register = scratchRegs[scratchIndex%len(scratchRegs)]

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

func generateTimeline(entryBlock *Block) []*Value {
	var order []*Block
	visited := make(map[int]struct{})

	var visit func(*Block)
	visit = func(b *Block) {
		if _, ok := visited[b.Id]; ok {
			return
		}
		visited[b.Id] = struct{}{}

		for _, succ := range b.Successors {
			visit(succ)
		}

		order = append(order, b)
	}

	visit(entryBlock)

	var globalTimeline []*Value

	for _, b := range slices.Backward(order) {
		globalTimeline = append(globalTimeline, b.Values...)
	}

	return globalTimeline
}
