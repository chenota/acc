package ssa

import (
	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
)

func BuildAndAllocate(program []*ir.Node) ([]*Func, error) {
	m := newModule()

	// declare every function for forward-reference capabaility
	for _, n := range program {
		if n.Op != ir.OpFunction {
			return nil, diagnostic.NewError(n.Pos, "expected function node")
		}
		// a lifted lambda has no symbol of its own, so functions are keyed by label
		m.declare(n.Signature.Label, n.Type)
	}

	// build every body
	for _, n := range program {
		if err := m.buildFuncBody(n); err != nil {
			return nil, err
		}
	}

	// run memory/register optimizations on unoptimized function bodies
	for _, f := range m.Funcs {
		mem2reg(f)
	}

	for _, f := range m.Funcs {
		// call escape summary to memoize it
		f.escapeSummary()
	}

	// heapify and optimize every function in the now-complete pool
	for _, f := range m.Funcs {
		heapify(f)
		if err := optimizeAndAllocate(f); err != nil {
			return nil, err
		}
	}

	return m.Funcs, nil
}

func optimizeAndAllocate(f *Func) error {
	unaryFold(f)
	quickFold(f)
	associativeFold(f)
	negSquash(f)
	lowerConstraints(f)
	spill(f)
	if err := regalloc(f); err != nil {
		return err
	}
	layoutFrame(f)

	return nil
}
