package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
)

func lowerConstraints(f *Func) {
	lowerDivides(f)
	lowerReturns(f)
}

func lowerReturns(f *Func) {
	for _, b := range f.Blocks {
		if b.Kind == BlockRet && b.Control != nil {
			b.Control.Loc = NewReg(register.ReturnTarget)
		}
	}
}

func lowerDivides(f *Func) {
	for _, b := range f.Blocks {
		for _, v := range slices.Clone(b.Values) {
			if v.Op != OpDivide {
				continue
			}

			copy := f.insertValueBefore(v, OpCopy, v.Args[0].Type, b)
			copy.Args = []*Value{v.Args[0]}
			v.Args[0] = copy

			v.Loc = NewReg(register.RegA)
			v.Clobbers = []register.Register{register.RegD}
		}
	}
}
