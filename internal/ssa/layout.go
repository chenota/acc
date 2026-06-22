package ssa

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
	f.frameSize = (offset + 15) &^ 15
}
