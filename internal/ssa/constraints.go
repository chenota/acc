package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/target"
)

// TODO: Apple arm64 ABI does stack arguments by natural alignment so this doesn't work there
const stackSlotSize = 8

func lowerConstraints(f *Func) error {
	// the ABI pins what crosses a call boundary
	lowerParams(f)
	lowerClosurePtr(f)
	lowerCalls(f)
	lowerCallResults(f)
	lowerResults(f)

	// the instruction set pins whatever is left
	lowerRegs(f)

	return nil
}

// lowerClosurePtr reads the incoming closure object out of the register the caller left it in.
func lowerClosurePtr(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpClosurePtr {
			continue
		}
		placeResult(f, v, NewReg(f.Target.ClosureContext))
	}
}

// lowerParams reads each parameter out of wherever the caller left it.
func lowerParams(f *Func) {
	for v := range f.UnorderedValues() {
		if v.Op != OpParam {
			continue
		}

		// parameters store their index in the value slot
		placeResult(f, v, f.incomingArgs().loc(v.Value.(int)))
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
		placeOperand(f, v, 0, v.Loc)
	}
}

// lowerRegs pins every operand and result that its instruction can only read or write in one register
func lowerRegs(f *Func) {
	for _, b := range f.Blocks {
		for _, v := range slices.Clone(b.Values) {
			// a phi's operands are read on the way out of its predecessors, which have nowhere to put a copy yet
			if v.Op == OpPhi {
				continue
			}

			regs := v.regs()

			for i, m := range regs.inputs {
				if r, ok := m.Only(); ok {
					placeOperand(f, v, i, NewReg(r))
				} else if a := v.Args[i]; a.Loc.Kind == LocRegister && !m.Contains(a.Loc.Reg) {
					// the allocator can meet any wider demand, unless the operand is stuck in a register v can't read
					v.setArg(i, copyTo(f, v, a, Location{}))
				}
			}

			if r, ok := regs.output.Only(); ok && v.Loc.Kind == LocNone {
				placeResult(f, v, NewReg(r))
			}
		}
	}
}

// placeOperand has v read its ith operand from a copy made at loc just before v reads it
func placeOperand(f *Func, v *Value, i int, loc Location) {
	v.setArg(i, copyTo(f, v, v.Args[i], loc))
}

// placeResult pins v's result to loc, then hands it to an unconstrained copy
func placeResult(f *Func, v *Value, loc Location) {
	v.Loc = loc
	if !f.hasUses(v) {
		return
	}

	out := f.insertValueAfter(v, OpCopy, v.Type, v.Block)
	// redirect before wiring up the arg so the copy does not point at itself
	f.redirectUses(v, out)
	out.setArgs(v)
}

func lowerCalls(f *Func) {
	for v := range f.UnorderedValues() {
		if !v.IsCall() {
			continue
		}

		callArgs := v.CallArgs()
		base := len(v.Args) - len(callArgs)

		// arguments past the registers are written to the outgoing area at the bottom of this function's frame
		for i := range callArgs {
			placeOperand(f, v, base+i, f.outgoingArgs().loc(i))
		}

		// the context register only has a meaning at the call itself, so place it last to keep it right next to the call
		if v.Op == OpClosureCall {
			placeOperand(f, v, 1, NewReg(f.Target.ClosureContext))
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
		placeResult(f, v, f.incomingResults(len(v.Args[0].CallArgs())).loc(v.Value.(int)))
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
