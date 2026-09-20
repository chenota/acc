package ssa

import (
	"github.com/chenota/acc/internal/register"
)

const stackSlotSize = 8

func lowerConstraints(f *Func) {
	lowerParams(f)
	lowerClosurePtr(f)
	lowerDivides(f)
	lowerCalls(f)
	lowerCallResults(f)
	// important that lowerResults runs after lowerCallResults so the hint lands on the copy a call's result is read out of
	lowerResults(f)
}

// lowerClosurePtr pins the incoming closure object to the register the caller left it in.
func lowerClosurePtr(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpClosurePtr {
			continue
		}
		v.Loc = NewReg(register.ClosureContext)
	}
}

func lowerParams(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpParam {
			continue
		}

		// parameters store their index in the value slot
		v.Loc = incomingArgs.loc(v.Value.(int))
	}
}

// lowerResults pins each return value to a result register
func lowerResults(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpResult {
			continue
		}

		// results store their index in the value slot
		v.Loc = results.loc(v.Value.(int))

		// move the returned value into the register it names
		v.Args[0] = copyTo(f, v, v.Args[0], v.Loc)
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

		lo := copyTo(f, v, dividend, NewReg(register.RegA))

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
		if !v.IsCall() {
			continue
		}

		callArgs := v.CallArgs()
		base := len(v.Args) - len(callArgs)

		// arguments past the registers are written to the outgoing area at the bottom of this function's frame
		for i, arg := range callArgs {
			v.Args[base+i] = copyTo(f, v, arg, outgoingArgs.loc(i))
		}

		// the context register only has a meaning at the call itself, so pin it last.
		if v.Op == OpClosureCall {
			v.Args[ClosureCallObject] = copyTo(f, v, v.Args[ClosureCallObject], NewReg(register.ClosureContext))
		}
	}
}

// lowerCallResults reads each value a call hands back out of the register the ABI left it in.
func lowerCallResults(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpCallResult {
			continue
		}

		// call results store their index in the value slot
		v.Loc = results.loc(v.Value.(int))

		// move the value somewhere unconstrained before the next call needs the register back
		copyOut(f, v)
	}
}

type abiSeq struct {
	regs []register.Register
	mem  func(n int) Location // home of the nth leaf past the registers, or nil when there is no such convention
}

var (
	// the caller writes past rsp, and the callee reads them back past its saved rbp.
	incomingArgs = abiSeq{regs: register.Args, mem: func(n int) Location { return NewFrame(incomingArgOffset(n)) }}
	outgoingArgs = abiSeq{regs: register.Args, mem: func(n int) Location { return NewOutgoing(n * stackSlotSize) }}

	// results have nowhere to go once the result registers run out
	results = abiSeq{regs: register.Results}
)

// loc is the home of the nth leaf in the sequence. Only call this for a leaf that fits.
func (s abiSeq) loc(n int) Location {
	if n < len(s.regs) {
		return NewReg(s.regs[n])
	}
	return s.mem(n - len(s.regs))
}

// fits reports whether the sequence has a home for the nth leaf.
func (s abiSeq) fits(n int) bool {
	return n < len(s.regs) || s.mem != nil
}

// overflow is how many of n leaves land past the registers.
func (s abiSeq) overflow(n int) int {
	return max(0, n-len(s.regs))
}

// incomingArgOffset returns the rbp-relative offset of the nth incoming stack argument.
func incomingArgOffset(n int) int {
	// 16 to account for saved rbp + return address
	return 16 + n*stackSlotSize
}

// copyTo inserts a copy of arg pinned to loc just before v.
func copyTo(f *Func, v *Value, arg *Value, loc Location) *Value {
	in := f.insertValueBefore(v, OpCopy, arg.Type, v.Block)
	in.Args = []*Value{arg}
	in.Loc = loc
	if loc.Kind == LocRegister {
		arg.RecordHint(loc.Reg) // try to put arg where v is to make this copy redundant
	}
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
