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
				Explanation of this lowering step for my own sanity:

				- The idiv instruction is stupid and clobbers a bunch of registers
				- Dividiend / Divisor -> Quotient & Remainder
				- Upper half of dividend is locked to RDX, lower half locked to RAX
				- Divisor can go anywhere but RAX and RDX (duh)
				- Results: Quotent goes in RAX, remainder goes in RDX

				- Current formulation: v.Args[0] is the dividend, v.Args[1] is the divisor
				- So we make a copy instruction for v.Args[0] pinned to RAX to force that value into RAX
				- This step is inefficient but exists for correctness; optimization passes can get rid of this later :)
			*/

			copy := f.insertValueBefore(v, OpCopy, v.Args[0].Type, b)
			copy.Args = []*Value{v.Args[0]}
			copy.Loc = NewReg(register.RegA)
			v.Args[0] = copy

			v.Loc = NewReg(register.RegA)
			v.Clobbers = []register.Register{register.RegA, register.RegD}
		}
	}
}
