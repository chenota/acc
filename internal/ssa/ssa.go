package ssa

import (
	"github.com/chenota/acc/internal/buildcfg"
	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/register"
)

type Option func(m *Module)

// WithTarget compiles for t instead of the target buildcfg resolved.
func WithTarget(t *register.Target) Option {
	return func(m *Module) {
		m.Target = t
	}
}

func BuildAndAllocate(program []*ir.Node, opts ...Option) ([]*Func, error) {
	m := &Module{byName: make(map[string]*Func)}

	for _, o := range opts {
		o(m)
	}

	if m.Target == nil {
		t, err := register.For(buildcfg.TargetArch, buildcfg.TargetOS)
		if err != nil {
			return nil, err
		}
		m.Target = t
	}

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

	for _, f := range m.Funcs {
		if err := runStages(f, stagesBeforeEscape); err != nil {
			return nil, err
		}
	}

	for _, f := range m.Funcs {
		// call escape summary to memoize it
		f.escapeSummary()
	}

	// every function in the pool is summarized, so each one can be heapified and optimized
	for _, f := range m.Funcs {
		if err := runStages(f, stagesAfterEscape); err != nil {
			return nil, err
		}
	}

	return m.Funcs, nil
}

// a stage is one step of the pipeline a function's body goes through
type stage struct {
	name string
	run  func(*Func) error
}

// allocateStage is the stage that hands out registers
const allocateStage = "allocate"

// stagesBeforeEscape tidy up a freshly built body; escape analysis needs these to run before it
var stagesBeforeEscape = []stage{
	// immediately prune unreachable blocks to prevent panics
	{"pruneUnreachable", pruneUnreachable},
	// run memory/register optimizations on unoptimized function bodies
	{"mem2reg", mem2reg},
}

// stagesAfterEscape place escaping slots on the heap, then optimize and allocate
var stagesAfterEscape = []stage{
	{"heapify", heapify},
	{"unaryFold", unaryFold},
	{"quickFold", quickFold},
	{"associativeFold", associativeFold},
	{"negSquash", negSquash},
	{"lowerConstraints", lowerConstraints},
	{allocateStage, allocate},
	{"layoutFrame", layoutFrame},
}

// runStages puts f through each of stages in order
func runStages(f *Func, stages []stage) error {
	for _, s := range stages {
		if err := runStage(f, s.name, s.run); err != nil {
			return err
		}
	}

	return nil
}
