package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
)

const stackSlotSize = 8

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

		// parameters store their index in the value slot
		i := v.Value.(int)
		// first 6 args arrive in registers
		if i < len(register.Args) {
			v.Loc = NewReg(register.Args[i])
			continue
		}
		// the rest arrive at the very bottom of the caller's frame
		v.Loc = NewFrame(incomingArgOffset(i - len(register.Args)))
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

		lo := copyIn(f, v, dividend, register.RegA)

		hi := f.insertValueBefore(v, OpSignExtend, dividend.Type, v.Block)
		hi.Args = []*Value{lo}
		hi.Loc = NewReg(register.RegD)

		v.Args = []*Value{lo, divisor, hi}
		v.Loc = NewReg(register.RegA)

		copyOut(f, v)
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
				v.Args[i+1] = copyIn(f, v, arg, register.Args[i])
				continue
			}
			// the rest are written to the outgoing area at the bottom of this function's frame
			v.Args[i+1] = copyToOutgoingStack(f, v, arg, i-len(register.Args))
		}

		v.Loc = NewReg(register.RegA)
		v.Clobbers = slices.Collect(register.CallerSaved.All())

		copyOut(f, v)
	}
}

// incomingArgOffset returns the rbp-relative offset of the nth incoming stack argument.
func incomingArgOffset(n int) int {
	// 16 to account for saved rbp + return address
	return 16 + n*stackSlotSize
}

// copyToOutgoingStack inserts a copy of arg into the nth slot of f's outgoing argument area.
func copyToOutgoingStack(f *Func, v *Value, arg *Value, n int) *Value {
	in := f.insertValueBefore(v, OpCopy, arg.Type, v.Block)
	in.Args = []*Value{arg}
	in.Loc = NewOutgoing(n * stackSlotSize)
	return in
}

// copyIn inserts a copy of arg pinned to reg just before v.
func copyIn(f *Func, v *Value, arg *Value, reg register.Register) *Value {
	in := f.insertValueBefore(v, OpCopy, arg.Type, v.Block)
	in.Args = []*Value{arg}
	in.Loc = NewReg(reg)
	arg.RecordHint(reg) // try to put arg where v is to make this copy redundant
	return in
}

// copyOut inserts an unconstrained copy of v's result just after v and points v's users at it.
func copyOut(f *Func, v *Value) *Value {
	out := f.insertValueAfter(v, OpCopy, v.Type, v.Block)
	// redirect before wiring up the arg so the copy does not point at itself
	f.redirectUses(v, out)
	out.Args = []*Value{v}
	if v.Loc.Kind == LocRegister {
		out.RecordHint(v.Loc.Reg) // try to put out where v is to make this copy redundant
	}
	return out
}
