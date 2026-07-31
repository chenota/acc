package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/types"
)

// heapify moves escaping slots off the frame.
func heapify(f *Func, escaped []*Slot) {
	for _, slot := range escaped {
		block, i := f.allocationPoint(slot)
		heapPtr := f.insertValueAt(i, OpAllocate, types.Pointer(slot.Type), block)

		for v := range f.SlotValues(slot) {
			switch v.Op {
			case OpLocalAddr:
				// the slot's address is now whatever the allocation handed back
				f.redirectUses(v, heapPtr)
				f.removeValue(v)
			case OpStaticLoad:
				v.Op = OpLoad
				v.Args = []*Value{heapPtr}
				v.Value = nil
			case OpStaticStore:
				v.Op = OpStore
				v.Args = []*Value{v.Args[0], heapPtr}
				v.Value = nil

				// ensure parameter prologue shenanigans happen after we allocate the space
				// TODO: rework parameter prologue so this is not necessary
				f.ensureAfter(v, heapPtr)
			}
		}

		f.Slots = slices.DeleteFunc(f.Slots, func(s *Slot) bool { return s == slot })
	}
}
