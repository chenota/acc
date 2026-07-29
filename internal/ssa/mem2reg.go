package ssa

import (
	"iter"
)

// mem2reg promotes scalar never-addressed slots to SSA values
func mem2reg(f *Func) {
	for slot := range promotableSlots(f) {
		var currentDef *Value

		for v := range f.OrderedValues() {
			if v.Slot() != slot {
				continue
			}
			switch v.Op {
			case OpStaticStore:
				// capture the most recent value stored to this slot and delete the store operation
				currentDef = v.Args[0]
				f.removeValue(v)
			case OpStaticLoad:
				// point users at the stored value and delete the load
				f.redirectUses(v, currentDef)
				f.removeValue(v)
			}
		}
	}
}

func promotableSlots(f *Func) iter.Seq[*Slot] {
	return func(yield func(*Slot) bool) {
		addressed := make(map[*Slot]struct{})

		for v := range f.UnorderedValues() {
			if v.Slot() == nil {
				continue
			}
			if v.Op != OpStaticLoad && v.Op != OpStaticStore {
				addressed[v.Slot()] = struct{}{}
			}
		}

		for _, s := range f.Slots {
			// non-scalar and addressed slots stay in memory
			if _, ok := addressed[s]; ok || !s.Type.IsScalar() {
				continue
			}
			// slot is promotable
			if !yield(s) {
				break
			}
		}
	}
}
