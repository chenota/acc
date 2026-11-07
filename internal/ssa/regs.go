package ssa

import (
	"github.com/chenota/acc/internal/buildcfg"
	"github.com/chenota/acc/internal/target"
)

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
func (v *Value) regs() regInfo {
	t := v.Block.Func.Target

	var info regInfo
	switch t.Arch {
	case buildcfg.ArchX64:
		info = x64Regs[v.Op]
	default:
		info = arm64Regs[v.Op]
	}

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

var (
	rax = target.NewMask(target.RegA)
	rdx = target.NewMask(target.RegD)
)

// opRegs lists any default usage register info about each op
type opRegs map[Op]regInfo

var x64Regs = opRegs{
	// x64 arithmetic writes its result over its left operand
	OpAdd:      {twoAddress: true},
	OpSubtract: {twoAddress: true},
	OpMultiply: {twoAddress: true},
	OpNegate:   {twoAddress: true},
	OpNot:      {twoAddress: true},

	// cdq/cqo sign-extends rax into rdx
	OpSignExtend: {inputs: []target.Mask{rax}, output: rdx},

	// idiv divides rdx:rax by its divisor, leaving the quotient in rax and the remainder in rdx
	OpDivide: {
		inputs:   []target.Mask{rax, anyReg.Without(rax.Union(rdx)), rdx},
		output:   rax,
		clobbers: rdx,
	},
}

var arm64Regs = opRegs{}
