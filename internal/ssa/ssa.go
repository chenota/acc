package ssa

import (
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/register"
)

var (
	defaultWorkingRegisters = []register.Register{register.Reg8, register.Reg9, register.Reg10, register.Reg11, register.Reg12, register.Reg13}
	defaultScratchRegisters = []register.Register{register.Reg14, register.Reg15}
	defaultReturnTarget     = register.RegA
)

func BuildAndAllocate(program []*ir.Node, opts ...Option) ([]*Func, error) {
	builder := &ssaBuilder{
		workingRegisters: defaultWorkingRegisters,
		scratchRegisters: defaultScratchRegisters,
		returnTarget:     defaultReturnTarget,
	}

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

	s.regalloc(f)

	return f, nil
}

type Option func(s *ssaBuilder)

func WithWorkingRegisters(regs []register.Register) Option {
	return func(s *ssaBuilder) {
		s.workingRegisters = regs
	}
}

func WithScratchRegisters(regs []register.Register) Option {
	return func(s *ssaBuilder) {
		s.scratchRegisters = regs
	}
}

func WithReturnRegister(r register.Register) Option {
	return func(s *ssaBuilder) {
		s.returnTarget = r
	}
}

type ssaBuilder struct {
	workingRegisters []register.Register
	scratchRegisters []register.Register
	returnTarget     register.Register
}
