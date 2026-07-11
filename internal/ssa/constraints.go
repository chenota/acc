package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
)

func lowerConstraints(f *Func) {
	lowerParams(f)
	lowerDivides(f)
	lowerCalls(f)
	// important that lowerReturns runs after lowerCalls so it targets the correct value
	lowerReturns(f)
}

func lowerParams(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpParam {
			continue
		}

		i := v.Value.(int)
		// first 6 args arrive in registers
		if i < len(register.Args) {
			v.Loc = NewReg(register.Args[i])
			continue
		}
		// TODO: args beyond the register count arrive on the stack
	}
}

func lowerReturns(f *Func) {
	for _, b := range f.Blocks {
		if b.Kind == BlockRet && b.Control != nil {
			b.Control.Loc = NewReg(register.ReturnTarget)
		}
	}
}

func lowerDivides(f *Func) {
	for v := range f.UnorderedValues() {
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

		lo := f.insertValueBefore(v, OpCopy, dividend.Type, v.Block)
		lo.Args = []*Value{dividend}
		lo.Loc = NewReg(register.RegA)

		hi := f.insertValueBefore(v, OpSignExtend, dividend.Type, v.Block)
		hi.Args = []*Value{lo}
		hi.Loc = NewReg(register.RegD)

		v.Args = []*Value{lo, divisor, hi}
		v.Loc = NewReg(register.RegA)
	}
}

func lowerCalls(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpCall {
			continue
		}

		for i, arg := range v.Args[1:] {
			// first 6 args go in registers
			if i < len(register.Args) {
				// copy the argument into its register
				copy := f.insertValueBefore(v, OpCopy, arg.Type, v.Block)
				copy.Args = []*Value{arg}
				copy.Loc = NewReg(register.Args[i])
				v.Args[i+1] = copy
				continue
			}
			// TODO: args in the stack go into special area in the stack
		}

		v.Loc = NewReg(register.RegA)
		v.Clobbers = slices.Collect(register.CallerSaved.All())

		// copy out RAX return value to new unconstrained value
		result := f.insertValueAfter(v, OpCopy, v.Type, v.Block)
		f.redirectUses(v, result)
		result.Args = []*Value{v}
	}
}
