package ssa

import "github.com/chenota/acc/internal/target"

// regInfo indicates what registers are available for an operation
type regInfo struct {
	inputs   []target.Mask // for each item in Args
	output   target.Mask
	clobbers target.Mask

	// twoAddress means the instruction overwrites its first operand with its result
	twoAddress bool
}

const anyReg = ^target.Mask(0)

// regs reports how v is able to interact with the register file
// TODO: might want to cache this
func (v *Value) regs() regInfo {
	t := v.Block.Func.Target
	info := v.Op.info().reg

	resolved := regInfo{
		inputs:     make([]target.Mask, len(v.Args)),
		clobbers:   info.clobbers,
		twoAddress: info.twoAddress,
	}

	for i := range v.Args {
		m := anyReg
		if i < len(info.inputs) {
			m = info.inputs[i]
		}
		resolved.inputs[i] = m.Intersect(t.Allocatable)
	}

	if v.NeedsRegister() {
		m := anyReg
		if info.output != 0 {
			m = info.output
		}
		resolved.output = m.Intersect(t.Allocatable)
	}

	if v.IsCall() {
		resolved.clobbers = resolved.clobbers.Union(t.CallerSaved)
	}

	return resolved
}

// allowed is every register v can live in
func (v *Value) allowed() target.Mask {
	// start with all available regs it can be written to
	m := v.regs().output
	for _, u := range v.users {
		inputs := u.regs().inputs
		for i, a := range u.Args {
			if a == v {
				// restrict to only registers its users can read from
				m = m.Intersect(inputs[i])
			}
		}
	}
	return m
}
