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
	blocks := linearizeBlocks(f.Entry)
	timeline := flattenValues(blocks)
	intervals := computeLiveIntervals(timeline)

	free := []string{"rax", "rbx"}
	active := []*liveInterval{}

	for _, currentInterval := range intervals {
		// expire old intervals by freeing their registers and untracking them
		active, free = expireOldIntervals(currentInterval, active, free)

		if len(free) == 0 {
			active = spillInterval(f, currentInterval, active)
		} else {
			active, free = takeFree(currentInterval, active, free)
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

func takeFree(curr *liveInterval, active []*liveInterval, free []string) ([]*liveInterval, []string) {
	reg := free[0]

	curr.Value.Register = reg
	active = append(active, curr)
	slices.SortFunc(active, func(a, b *liveInterval) int { return a.End - b.End })

	return active, free[1:]
}

func spillInterval(f *Func, curr *liveInterval, active []*liveInterval) []*liveInterval {
	spill := active[len(active)-1]

	// we have a very long-lived value it should get stack-ed
	if curr.End > spill.End {
		injectSpill(f, curr.Value)
		return active
	}

	curr.Value.Register = spill.Value.Register
	injectSpill(f, spill.Value)

	active[len(active)-1] = curr
	slices.SortFunc(active, func(a, b *liveInterval) int { return a.End - b.End })

	return active
}

func expireOldIntervals(curr *liveInterval, active []*liveInterval, free []string) ([]*liveInterval, []string) {
	for i, interval := range active {
		if interval.End >= curr.Start {
			return active[i:], free
		}
		free = append(free, interval.Value.Register)
	}
	return nil, free
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

func linearizeBlocks(entryBlock *Block) []*Block {
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

	slices.Reverse(order)

	return order
}

func flattenValues(orderedBlocks []*Block) []*Value {
	var globalTimeline []*Value

	for _, b := range orderedBlocks {
		globalTimeline = append(globalTimeline, b.Values...)
	}

	return globalTimeline
}
