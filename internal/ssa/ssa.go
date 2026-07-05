package ssa

import (
	"github.com/chenota/acc/internal/ir"
)

func BuildAndAllocate(program []*ir.Node) ([]*Func, error) {
	var compiledFuncs []*Func

	for _, n := range program {
		f, err := optimizedAllocatedFunction(n)
		if err != nil {
			return nil, err
		}
		compiledFuncs = append(compiledFuncs, f)
	}

	return compiledFuncs, nil
}

func optimizedAllocatedFunction(n *ir.Node) (*Func, error) {
	f, err := buildFunc(n)
	if err != nil {
		return nil, err
	}
	mem2reg(f)
	unaryFold(f)
	quickFold(f)
	associativeFold(f)
	negSquash(f)
	lowerConstraints(f)
	spill(f)
	if err := regalloc(f); err != nil {
		return nil, err
	}
	layoutFrame(f)

	return f, nil
}
