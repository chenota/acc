package ssa

// layoutFrame assigns each alloca a slot in the stack frame, growing downward from rbp.
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
}
