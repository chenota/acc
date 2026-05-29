package ssa

import "github.com/chenota/acc/internal/ir"

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
	f, err := initialFunc(n)
	if err != nil {
		return nil, err
	}

	return f, nil
}
