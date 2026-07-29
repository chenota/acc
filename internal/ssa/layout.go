package ssa

import "slices"

// layoutFrame drops the slots no value names any more and gives the survivors a home in the stack frame.
func layoutFrame(f *Func) {
	f.Slots = namedSlots(f)

	// widest slots first
	slices.SortStableFunc(f.Slots, func(a, b *Slot) int {
		return b.Type.Size() - a.Type.Size()
	})

	var offset int
	for _, s := range f.Slots {
		byteSize := s.Type.Size()
		offset += byteSize
		offset = (offset + byteSize - 1) &^ (byteSize - 1)
		s.Loc = NewFrame(-offset)
	}
}

// namedSlots returns the slots some value still reads or writes
func namedSlots(f *Func) []*Slot {
	named := make(map[*Slot]struct{})
	for v := range f.UnorderedValues() {
		if s := v.Slot(); s != nil {
			named[s] = struct{}{}
		}
	}

	var live []*Slot
	for _, s := range f.Slots {
		if _, ok := named[s]; ok {
			live = append(live, s)
		}
	}
	return live
}
