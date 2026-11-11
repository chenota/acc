package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/target"
	"github.com/chenota/acc/internal/types"
)

// signatureLocations is where each leaf of a signature's parameters and results is passed
type signatureLocations struct {
	params, results []Location
	stackBytes      int // size of the stack area holding leaves that must overflow to the stack
}

// locationsAsCallee is where f finds its own parameters and leaves its own results.
func (f *Func) locationsAsCallee() signatureLocations {
	// the caller's stack area starts just above f's frame record
	return f.locateSignature(f.Params, f.Results, f.frameLoc(f.Target.FrameRecordSize))
}

// locationsAsCaller is where f puts the arguments of call and finds its results.
func (f *Func) locationsAsCaller(call *Value) signatureLocations {
	params := make([]*types.Type, len(call.CallArgs()))
	for i, arg := range call.CallArgs() {
		params[i] = arg.Type
	}
	results := slices.Collect(iterutil.Second(call.Type.Leaves()))

	// f sets the stack area up at the bottom of its own frame
	return f.locateSignature(params, results, f.outgoingLoc(0))
}

// locateSignature passes params and results in registers, and packs the rest into the stack area starting at area, results above params.
func (f *Func) locateSignature(params, results []*types.Type, area Location) signatureLocations {
	leaves := slices.Concat(params, results)
	locs := slices.Concat(assignRegisters(len(params), f.Target.Args), assignRegisters(len(results), f.Target.Results))

	slot := f.Target.StackArgSlotSize
	var used int
	for i, t := range leaves {
		if locs[i].Kind != LocNone {
			continue
		}
		offset := roundUp(used, max(t.Align(), slot))
		locs[i] = NewMem(area.Reg, area.Offset+offset)
		used = offset + roundUp(t.Size(), slot)
	}

	return signatureLocations{params: locs[:len(params)], results: locs[len(params):], stackBytes: used}
}

// assignRegisters puts the first n leaves in regs in order, leaving any past the end of regs LocNone.
func assignRegisters(n int, regs []target.Register) []Location {
	locs := make([]Location, n)
	for i := range min(n, len(regs)) {
		locs[i] = NewReg(regs[i])
	}
	return locs
}
