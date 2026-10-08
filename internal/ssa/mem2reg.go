package ssa

import (
	"iter"
)

// mem2reg promotes never-addressed slots to SSA values, one leaf at a time
func mem2reg(f *Func) {
	for slot := range promotableSlots(f) {
		// the most recent value stored to each leaf
		currentDefs := make(map[int]*Value)

		for v := range f.OrderedValues() {
			if v.Slot() != slot {
				continue
			}
			switch v.Op {
			case OpStaticStore:
				// capture the most recent value stored to this leaf and delete the store operation
				currentDefs[v.Offset] = v.Args[0]
				f.removeValue(v)
			case OpStaticLoad:
				// point users at the value stored to this leaf and delete the load
				f.redirectUses(v, currentDefs[v.Offset])
				f.removeValue(v)
			}
		}
	}
}

func promotableSlots(f *Func) iter.Seq[*Slot] {
	return func(yield func(*Slot) bool) {
		pinned := make(map[*Slot]struct{})

		for v := range f.UnorderedValues() {
			// anything other than a static load and store of a slot pins to memory
			if s := v.Slot(); s != nil && v.Op != OpStaticLoad && v.Op != OpStaticStore {
				pinned[s] = struct{}{}
			}
		}

		for slot := range pinned {
			if !yield(slot) {
				break
			}
		}
	}
}
