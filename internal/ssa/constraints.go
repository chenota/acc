package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/target"
)

// TODO: Apple arm64 ABI does stack arguments by natural alignment so this doesn't work there
const stackSlotSize = 8

// lowerConstraints pins everything that must be pinned, and otherwise leaves it up to the allocator
func lowerConstraints(f *Func) error {
	for _, b := range f.Blocks {
		for _, v := range slices.Clone(b.Values) {
			// a phi's operands are read on the way out of its predecessors, which have nowhere to put a copy yet
			if v.Op == OpPhi {
				continue
			}

			abi, regs := f.abi(v), v.regs()

			for i, loc := range abi.operands {
				if r, ok := regs.inputs[i].Only(); ok && loc.Kind == LocNone {
					loc = NewReg(r)
				}
				if loc.Kind != LocNone {
					placeOperand(f, v, i, loc)
				}
			}

			loc := abi.result
			if r, ok := regs.output.Only(); ok && loc.Kind == LocNone {
				loc = NewReg(r)
			}
			if loc.Kind != LocNone {
				placeResult(f, v, loc)
			}
		}
	}

	return nil
}

// placement is where the calling convention puts a value's operands and result
type placement struct {
	operands []Location
	result   Location
}

// abi is where the calling convention puts v's operands and result
func (f *Func) abi(v *Value) placement {
	p := placement{operands: make([]Location, len(v.Args))}

	// parameters, results, and call results store their index in the value slot
	switch v.Op {
	case OpParam:
		p.result = f.incomingArgs().loc(v.Value.(int))
	case OpClosurePtr:
		p.result = NewReg(f.Target.ClosureContext)
	case OpCallResult:
		p.result = f.incomingResults(len(v.Args[0].CallArgs())).loc(v.Value.(int))
	case OpResult:
		// where the overflow results end up depends on how many argument leaves the signature has
		p.operands[0] = f.outgoingResults(f.paramLeaves()).loc(v.Value.(int))
	}

	if v.IsCall() {
		// arguments past the registers are written to the outgoing area at the bottom of this function's frame
		base := len(v.Args) - len(v.CallArgs())
		for i := range v.CallArgs() {
			p.operands[base+i] = f.outgoingArgs().loc(i)
		}
		if v.Op.info().closureCall {
			p.operands[1] = NewReg(f.Target.ClosureContext)
		}
	}

	return p
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
	return f.frameLoc(f.incomingArgOffset(n))
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

// incomingArgOffset returns the frame-pointer-relative offset of the nth incoming stack argument.
func (f *Func) incomingArgOffset(n int) int {
	return f.Target.FrameRecordSize + n*stackSlotSize
}

// copyTo inserts a copy of arg pinned to loc just before v.
func copyTo(f *Func, v *Value, arg *Value, loc Location) *Value {
	in := f.insertValueBefore(v, OpCopy, arg.Type, v.Block)
	in.setArgs(arg)
	in.Loc = loc
	return in
}
