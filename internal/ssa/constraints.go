package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/target"
)

// lowerConstraints moves every operand and result that has a required location there through a copy
func lowerConstraints(f *Func) error {
	for _, b := range f.Blocks {
		for _, v := range slices.Clone(b.Values) {
			// a phi's operands are read on the way out of its predecessors, which have nowhere to put a copy yet
			if v.Op == OpPhi {
				continue
			}

			operands, result := f.requiredLocations(v)
			for i, loc := range operands {
				if loc.Kind != LocNone {
					copyOperandTo(f, v, i, loc)
				}
			}
			if result.Kind != LocNone {
				copyResultFrom(f, v, result)
			}
		}
	}

	return nil
}

// requiredLocations is where v's operands and result have to be
func (f *Func) requiredLocations(v *Value) (operands []Location, result Location) {
	// anything that fits in only one register has to go there
	regs := v.regs()
	operands = make([]Location, len(v.Args))
	for i, m := range regs.inputs {
		operands[i] = onlyRegister(m)
	}
	result = onlyRegister(regs.output)

	// the calling convention overrides that; params, results, and call results store their index in the value slot
	switch v.Op {
	case OpParam:
		result = f.locationsAsCallee().params[v.Value.(int)]
	case OpResult:
		operands[0] = f.locationsAsCallee().results[v.Value.(int)]
	case OpCallResult:
		result = f.locationsAsCaller(v.Args[0]).results[v.Value.(int)]
	case OpClosurePtr:
		result = NewReg(f.Target.ClosureContext)
	}

	if v.IsCall() {
		// the arguments are the last operands, after a closure call's code pointer and context
		first := len(v.Args) - len(v.CallArgs())
		copy(operands[first:], f.locationsAsCaller(v).params)
		if v.Op.info().closureCall {
			operands[1] = NewReg(f.Target.ClosureContext)
		}
	}

	return operands, result
}

// onlyRegister is the one register m allows, or LocNone unless there is exactly one.
func onlyRegister(m target.Mask) Location {
	if r, ok := m.Only(); ok {
		return NewReg(r)
	}
	return Location{}
}

// copyOperandTo has v read its ith operand from a copy at loc made just before v
func copyOperandTo(f *Func, v *Value, i int, loc Location) {
	cp := f.insertValueBefore(v, OpCopy, v.Args[i].Type, v.Block)
	cp.setArgs(v.Args[i])
	cp.Loc = loc
	v.setArg(i, cp)
}

// copyResultFrom fixes v's result at loc and has v's users read a copy of it
func copyResultFrom(f *Func, v *Value, loc Location) {
	v.Loc = loc
	if !f.hasUses(v) {
		return
	}

	cp := f.insertValueAfter(v, OpCopy, v.Type, v.Block)
	// redirect before wiring up the arg so the copy does not point at itself
	f.redirectUses(v, cp)
	cp.setArgs(v)
}
