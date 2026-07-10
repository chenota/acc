package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
)

func calleeSaved(f *Func) {
	// registers the allocator used that this function must preserve for its caller
	saved := f.UsedRegisters() & register.CalleeSaved
	if saved.Count() == 0 {
		return
	}

	// push at the front of the entry block
	for reg := range saved.All() {
		push := f.newValue(OpPush, nil, f.Entry)
		push.Loc = NewReg(reg)
		f.Entry.Values = slices.Insert(f.Entry.Values, 0, push)
	}

	// pop at the end of every return block
	// pushes were inserted at the front which naturally reverses them, so do pops in order
	for _, b := range f.Blocks {
		if b.Kind != BlockRet {
			continue
		}
		for reg := range saved.All() {
			pop := f.newValue(OpPop, nil, b)
			pop.Loc = NewReg(reg)
			b.Values = append(b.Values, pop)
		}
	}
}
