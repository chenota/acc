package ssa

import (
	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
)

func BuildAndAllocate(program []*ir.Node) ([]*Func, error) {
	m := newModule()

	// declare every top-level function for forward-reference capabaility
	for _, n := range program {
		if n.Op != ir.OpFunction {
			return nil, diagnostic.NewError(n.Pos, "expected function node")
		}
		m.declare(n.Sym.Name)
	}

	// build every body. this is looking ahead a bit but basically this will eventually allow all lambdas to get added to the module
	for _, n := range program {
		if err := m.buildFuncBody(n); err != nil {
			return nil, err
		}
	}

	// optimize and allocate every function in the now-complete pool
	for _, f := range m.Funcs {
		if err := optimizeAndAllocate(f); err != nil {
			return nil, err
		}
	}

	return m.Funcs, nil
}

func optimizeAndAllocate(f *Func) error {
	mem2reg(f)
	unaryFold(f)
	quickFold(f)
	associativeFold(f)
	negSquash(f)
	lowerConstraints(f)
	spill(f)
	if err := regalloc(f); err != nil {
		return err
	}
	calleeSaved(f)
	layoutFrame(f)
	return nil
}
