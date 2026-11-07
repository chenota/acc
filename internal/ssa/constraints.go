package ssa

import (
	"github.com/chenota/acc/internal/target"
)

const stackSlotSize = 8

func lowerConstraints(f *Func) error {
	lowerParams(f)
	lowerClosurePtr(f)
	lowerDivides(f)
	lowerCalls(f)
	lowerCallResults(f)
	lowerResults(f)

	return nil
}

// lowerClosurePtr pins the incoming closure object to the register the caller left it in.
func lowerClosurePtr(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpClosurePtr {
			continue
		}
		v.Loc = NewReg(f.Target.ClosureContext)
	}
}

func lowerParams(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpParam {
			continue
		}

		// parameters store their index in the value slot
		v.Loc = f.incomingArgs().loc(v.Value.(int))
	}
}

// lowerResults pins each return value to a result register
func lowerResults(f *Func) {
	// where the overflow results end up depends on how many argument leaves the signature has
	seq := f.outgoingResults(f.paramLeaves())

	for v := range f.UnorderedValues() {
		if v.Op != OpResult {
			continue
		}

		// results store their index in the value slot
		v.Loc = seq.loc(v.Value.(int))

		// move the returned value into the place it names
		v.setArg(0, copyTo(f, v, v.Args[0], v.Loc))
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

		lo := copyTo(f, v, dividend, NewReg(target.RegA))

		hi := f.insertValueBefore(v, OpSignExtend, dividend.Type, v.Block)
		hi.setArgs(lo)
		hi.Loc = NewReg(target.RegD)

		v.setArgs(lo, divisor, hi)
		v.Loc = NewReg(target.RegA)

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
			v.setArg(base+i, copyTo(f, v, arg, f.outgoingArgs().loc(i)))
		}

		// the context register only has a meaning at the call itself, so pin it last.
		if v.Op == OpClosureCall {
			v.setArg(1, copyTo(f, v, v.Args[1], NewReg(f.Target.ClosureContext)))
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
		v.Loc = f.incomingResults(len(v.Args[0].CallArgs())).loc(v.Value.(int))

		// move the value somewhere unconstrained before the next call needs the register back
		copyOut(f, v)
	}
}

type abiSeq struct {
	regs []target.Register
	mem  func(n int) Location // home of the nth leaf past the registers
}

// incomingArgs is where f finds its own arguments; the ones past the registers are read back past its saved frame pointer.
func (f *Func) incomingArgs() abiSeq {
	return abiSeq{regs: f.Target.Args, mem: f.calleeView}
}

// outgoingArgs is where f leaves the arguments of a call it makes; the ones past the registers are written past the stack pointer.
func (f *Func) outgoingArgs() abiSeq {
	return abiSeq{regs: f.Target.Args, mem: f.callerView}
}

// outgoingResults is the callee's view of where it leaves the results of a call taking nArgs argument leaves.
func (f *Func) outgoingResults(nArgs int) abiSeq {
	return abiSeq{regs: f.Target.Results, mem: shift(f.calleeView, f.outgoingArgs().overflow(nArgs))}
}

// incomingResults is the caller's view of those same slots.
func (f *Func) incomingResults(nArgs int) abiSeq {
	return abiSeq{regs: f.Target.Results, mem: shift(f.callerView, f.outgoingArgs().overflow(nArgs))}
}

// calleeView addresses the nth slot of the region from inside the callee, past its saved frame pointer.
func (f *Func) calleeView(n int) Location {
	return f.frameLoc(incomingArgOffset(n))
}

// callerView addresses the nth slot of the same region from the caller, at the bottom of its frame.
func (f *Func) callerView(n int) Location {
	return f.outgoingLoc(n * stackSlotSize)
}

// shift moves a view along by base slots.
func shift(view func(int) Location, base int) func(int) Location {
	return func(n int) Location { return view(base + n) }
}

// loc is the home of the nth leaf in the sequence.
func (s abiSeq) loc(n int) Location {
	if n < len(s.regs) {
		return NewReg(s.regs[n])
	}
	return s.mem(n - len(s.regs))
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
	in.setArgs(arg)
	in.Loc = loc
	return in
}

// copyOut inserts an unconstrained copy of v's result just after v and points v's users at it.
func copyOut(f *Func, v *Value) *Value {
	out := f.insertValueAfter(v, OpCopy, v.Type, v.Block)
	// redirect before wiring up the arg so the copy does not point at itself
	f.redirectUses(v, out)
	out.setArgs(v)
	return out
}
