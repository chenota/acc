package codegen

import (
	"errors"
	"fmt"
	"slices"

	"github.com/chenota/acc/internal/buildcfg"
	"github.com/chenota/acc/internal/ssa"
)

type config struct {
	arch buildcfg.Arch
	os   buildcfg.OS
}

type Option func(c *config)

func WithTarget(arch buildcfg.Arch, os buildcfg.OS) Option {
	return func(c *config) {
		c.arch = arch
		c.os = os
	}
}

// GenerateAssembly lowers allocated SSA functions to assembly text for the target architecture.
func GenerateAssembly(program []*ssa.Func, opts ...Option) (string, error) {
	c := config{arch: buildcfg.TargetArch(), os: buildcfg.TargetOS()}
	for _, o := range opts {
		o(&c)
	}

	switch c.arch {
	case buildcfg.ArchX64:
		p, err := generateX64(program, symbols{os: c.os})
		if err != nil {
			return "", err
		}
		return p.String(), nil
	default:
		return "", fmt.Errorf("no code generator for %s", c.arch)
	}
}

// arch is everything an architecture supplies to turn SSA into its instructions.
type arch[I fmt.Stringer] interface {
	prologue(e *emitter[I], f *ssa.Func)
	value(e *emitter[I], v *ssa.Value) error
	move(e *emitter[I], m move)
	terminator(e *emitter[I], f *ssa.Func, b, next *ssa.Block)
	epilogue(e *emitter[I], f *ssa.Func)
}

// emitter appends instructions to the block a function is currently laying out.
type emitter[I fmt.Stringer] struct {
	symbols
	fn  *Func[I]
	cur *Block[I]
}

func (e *emitter[I]) emit(insts ...I) {
	e.cur.Insts = append(e.cur.Insts, insts...)
}

func (e *emitter[I]) startBlock(label string) {
	e.cur = &Block[I]{Label: label}
	e.fn.Blocks = append(e.fn.Blocks, e.cur)
}

func generate[I fmt.Stringer](a arch[I], s symbols, program []*ssa.Func) (*Program[I], error) {
	if !slices.ContainsFunc(program, (*ssa.Func).IsMain) {
		return nil, errors.New("program has no main function")
	}

	p := &Program[I]{}
	for _, f := range program {
		fn, err := generateFunction(a, s, f)
		if err != nil {
			return nil, err
		}
		p.Funcs = append(p.Funcs, fn)
	}

	return p, nil
}

func generateFunction[I fmt.Stringer](a arch[I], s symbols, f *ssa.Func) (*Func[I], error) {
	e := &emitter[I]{symbols: s, fn: &Func[I]{Name: s.funcLabel(f), Global: true}}

	// the prologue runs on entry under the function's own symbol
	e.startBlock("")
	a.prologue(e, f)

	blocks := slices.Collect(f.OrderedBlocks())
	for i, b := range blocks {
		// next is the block laid out right after b
		var next *ssa.Block
		if i+1 < len(blocks) {
			next = blocks[i+1]
		}

		e.startBlock(e.blockLabel(f, b))
		for _, v := range b.Values {
			if err := a.value(e, v); err != nil {
				return nil, err
			}
		}

		// the successor's phis take their operands from b on the way in
		if b.Kind == ssa.BlockPlain {
			for _, m := range edgeMoves(b, b.Successors[0]) {
				a.move(e, m)
			}
		}

		a.terminator(e, f, b, next)
	}

	// every return block ends up here
	e.startBlock(e.epilogueLabel(f))
	a.epilogue(e, f)

	return e.fn, nil
}
