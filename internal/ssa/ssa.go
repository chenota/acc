package ssa

import (
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/register"
)

var defaultRegisterGroup = registerGroup{
	working:      []register.Register{register.Reg8, register.Reg9, register.Reg10, register.Reg11, register.Reg12, register.Reg13},
	scratch:      []register.Register{register.Reg14, register.Reg15},
	returnTarget: register.RegA,
}

func BuildAndAllocate(program []*ir.Node, opts ...Option) ([]*Func, error) {
	builder := &ssaBuilder{registers: defaultRegisterGroup}

	for _, o := range opts {
		o(builder)
	}

	var compiledFuncs []*Func

	for _, n := range program {
		f, err := builder.optimizedAllocatedFunction(n)
		if err != nil {
			return nil, err
		}
		compiledFuncs = append(compiledFuncs, f)
	}

	return compiledFuncs, nil
}

func (s *ssaBuilder) optimizedAllocatedFunction(n *ir.Node) (*Func, error) {
	f, err := buildFunc(n)
	if err != nil {
		return nil, err
	}
	unaryFold(f)
	quickFold(f)
	associativeFold(f)
	regalloc(f, s.registers)
	layoutFrame(f)

	return f, nil
}

type Option func(s *ssaBuilder)

func WithWorkingRegisters(regs ...register.Register) Option {
	return func(s *ssaBuilder) {
		s.registers.working = regs
	}
}

func WithScratchRegisters(regs ...register.Register) Option {
	return func(s *ssaBuilder) {
		s.registers.scratch = regs
	}
}

func WithReturnRegister(r register.Register) Option {
	return func(s *ssaBuilder) {
		s.registers.returnTarget = r
	}
}

type ssaBuilder struct {
	registers registerGroup
}

type registerGroup struct {
	working      []register.Register
	scratch      []register.Register
	returnTarget register.Register
}
