package ssa

// redundantCopies removes redundant copy operations from f
func redundantCopies(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op == OpCopy && v.Args[0].Loc == v.Loc {
			f.redirectUses(v, v.Args[0])
			f.removeValue(v)
		}
	}
}
