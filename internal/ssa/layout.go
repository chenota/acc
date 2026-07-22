package ssa

import "github.com/chenota/acc/internal/register"

// layoutFrame assigns stack frame offets to values that need one
func layoutFrame(f *Func) {
	var offset int
	for _, v := range f.Entry.Values {
		if v.Op != OpAlloca {
			continue
		}

		byteSize := v.Type.Size()
		offset += byteSize
		offset = (offset + byteSize - 1) &^ (byteSize - 1)
		v.Loc = NewFrame(-offset)
	}

	// three regions of the frame that shift rsp:
	// 1. Callee-saved registers (push operation so don't want to account for frame size beyond aligning it, this is a silly design)
	// 2. Local alloca's
	// 3. Outgoing argument area for args on the stack
	pushBytes := (f.UsedRegisters() & register.CalleeSaved).Count() * 8
	f.frameSize = ((pushBytes + offset + f.maxOutgoingSize() + 15) &^ 15) - pushBytes
}
