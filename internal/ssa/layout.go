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
		v.Loc = NewStack(-offset)
	}

	frame := (offset + 15) &^ 15

	// Callee-saved registers are pushed in the previous step. Align frame size to 16 bytes if odd number of pushes.
	if (f.UsedRegisters()&register.CalleeSaved).Count()%2 == 1 {
		frame += 8
	}

	f.frameSize = frame
}
