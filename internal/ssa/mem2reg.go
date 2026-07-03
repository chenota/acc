package ssa

import (
	"iter"
)

// mem2reg promotes scalar never-addressed memory values to SSA values
func mem2reg(f *Func) {
	for alloca := range promotableAllocas(f) {
		var currentDef *Value

		for _, v := range f.OrderedValues() {
			if v.Op == OpStore && v.ArgIndex(alloca) > -1 {
				// capture the most recent value stored to this alloca and delete the store operation
				currentDef = v.Args[0]
				f.removeValue(v)
			} else if v.Op == OpLoad && v.ArgIndex(alloca) > -1 {
				// replace load with direct use and remove old load operation
				f.substituteValue(v, currentDef)
				f.removeValue(v)
			}
		}
	}
}

func promotableAllocas(f *Func) iter.Seq[*Value] {
	return func(yield func(*Value) bool) {
	ArgsLoop:
		for v := range f.Args() {
			// value must be an alloca
			if v.Op != OpAlloca {
				continue
			}
			// non-scalar values must stay in memory
			if !v.Type.IsScalar() {
				continue
			}
			// v must only be used as an argument to load or store
			for user := range f.UnorderedValues() {
				i := user.ArgIndex(v)
				if i == -1 {
					continue
				}
				if !(user.Op == OpLoad || (user.Op == OpStore && i == 1)) {
					continue ArgsLoop
				}
			}
			if !yield(v) {
				break
			}
		}
	}
}
