package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/types"
)

// heapify moves escaping slots off the frame, handing each one to the runtime allocator.
func heapify(f *Func, escaped []*Slot) {
	for _, slot := range escaped {
		block, i := f.allocationPoint(slot)

		size := f.insertValueAt(i, OpLiteral, types.Int(), block)
		size.Value = int32(slot.Type.Size())

		heapPtr := f.insertValueAt(i+1, OpStaticCall, types.Pointer(slot.Type), block)
		heapPtr.Value = Alloc
		heapPtr.Args = []*Value{size}

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
			}
		}

		f.Slots = slices.DeleteFunc(f.Slots, func(s *Slot) bool { return s == slot })
	}
}
