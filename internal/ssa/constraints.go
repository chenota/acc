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

			/*
				Explanation of idiv since it kind of drove me crazy:
				- idiv divides a double-width dividend by a single-width divisor
				- Dividend / Divisor -> Quotient (RAX) & Remainder (RDX)
				- Lower half locked to RAX, upper half locked to RDX
				- Divisor can go anywhere but RAX and RDX (duh)
			*/

			dividend := v.Args[0]
			divisor := v.Args[1]

			lo := f.insertValueBefore(v, OpCopy, dividend.Type, b)
			lo.Args = []*Value{dividend}
			lo.Loc = NewReg(register.RegA)

			hi := f.insertValueBefore(v, OpSignExtend, dividend.Type, b)
			hi.Args = []*Value{lo}
			hi.Loc = NewReg(register.RegD)

			v.Args = []*Value{lo, divisor, hi}
			v.Loc = NewReg(register.RegA)
		}
	}
}
