//go:build ssacheck

package ssa

import (
	"fmt"
	"slices"

	"github.com/chenota/acc/internal/types"
)

// runStage runs the stage called name over f, then sanity-checks its result. Used for testing only.
func runStage(f *Func, name string, run func(*Func) error) error {
	if err := run(f); err != nil {
		return err
	}

	if err := verify(f); err != nil {
		return fmt.Errorf("%s: invalid ssa after %s: %w", f.Name(), name, err)
	}

	return nil
}

// verify reports the first broken invariant it finds in f
func verify(f *Func) error {
	if err := verifyBlocks(f); err != nil {
		return err
	}
	return verifyValues(f)
}

// verifyBlocks checks the shape of f's control flow graph
func verifyBlocks(f *Func) error {
	if !slices.Contains(f.Blocks, f.Entry) {
		return fmt.Errorf("entry block %d is not in the function", f.Entry.Id)
	}
	if len(f.Entry.Predecessors) > 0 {
		return fmt.Errorf("entry block %d has predecessors", f.Entry.Id)
	}

	reachable := slices.Collect(f.OrderedBlocks())

	for _, b := range f.Blocks {
		if !slices.Contains(reachable, b) {
			return fmt.Errorf("block %d is unreachable", b.Id)
		}

		// both ends of every edge have to agree on it, as many times as it's taken
		for _, s := range b.Successors {
			if !slices.Contains(f.Blocks, s) {
				return fmt.Errorf("block %d has successor %d outside the function", b.Id, s.Id)
			}
			if count(b.Successors, s) != count(s.Predecessors, b) {
				return fmt.Errorf("edge %d -> %d is missing from %d's predecessors", b.Id, s.Id, s.Id)
			}

			// copies on an edge need a block of their own to go in
			if len(b.Successors) > 1 && len(s.Predecessors) > 1 {
				return fmt.Errorf("edge %d -> %d is critical", b.Id, s.Id)
			}
		}
		for _, p := range b.Predecessors {
			if !slices.Contains(f.Blocks, p) {
				return fmt.Errorf("block %d has predecessor %d outside the function", b.Id, p.Id)
			}
			if count(p.Successors, b) != count(b.Predecessors, p) {
				return fmt.Errorf("edge %d -> %d is missing from %d's successors", p.Id, b.Id, p.Id)
			}
		}

		if err := verifyTerminator(b); err != nil {
			return err
		}
	}

	return nil
}

// verifyTerminator checks that b's kind matches how it hands off control
func verifyTerminator(b *Block) error {
	switch b.Kind {
	case BlockPlain:
		if len(b.Successors) != 1 || len(b.Control) != 0 {
			return fmt.Errorf("plain block %d has %d successors and %d control values", b.Id, len(b.Successors), len(b.Control))
		}
	case BlockIf:
		if len(b.Successors) != 2 || len(b.Control) != 1 {
			return fmt.Errorf("if block %d has %d successors and %d control values", b.Id, len(b.Successors), len(b.Control))
		}
		if c := b.Control[0]; c != nil && !types.Equal(types.Bool(), c.Type) {
			return fmt.Errorf("if block %d branches on v%d, which is not a bool", b.Id, c.Id)
		}
	case BlockRet:
		if len(b.Successors) != 0 {
			return fmt.Errorf("return block %d has %d successors", b.Id, len(b.Successors))
		}
		for _, c := range b.Control {
			if c != nil && c.Op != OpResult {
				return fmt.Errorf("return block %d hands back v%d, which is not a result", b.Id, c.Id)
			}
		}
	default:
		return fmt.Errorf("block %d has no kind", b.Id)
	}

	return nil
}

// verifyValues checks that every value sits in exactly one block, and that everything it reads is defined before it
func verifyValues(f *Func) error {
	dom := computeDominators(f)

	// where each value in the instruction stream sits
	type site struct {
		block *Block
		index int
	}
	sites := make(map[*Value]site)
	ids := make(map[int]*Value)

	for _, b := range f.Blocks {
		for i, v := range b.Values {
			if _, ok := sites[v]; ok {
				return fmt.Errorf("v%d appears more than once", v.Id)
			}
			if other, ok := ids[v.Id]; ok {
				return fmt.Errorf("v%d is the id of two different values (%v and %v)", v.Id, other.Op, v.Op)
			}
			if v.Block != b {
				return fmt.Errorf("v%d sits in block %d but thinks it's in %s", v.Id, b.Id, blockName(v.Block))
			}
			if s := v.Slot(); s != nil && !slices.Contains(f.Slots, s) {
				return fmt.Errorf("v%d names a slot the function doesn't have", v.Id)
			}
			sites[v] = site{block: b, index: i}
			ids[v.Id] = v
		}
	}

	// defined reports whether a is available to something reading it at index i of b
	defined := func(a *Value, b *Block, i int) error {
		if a == nil {
			return fmt.Errorf("a nil value is read in block %d", b.Id)
		}
		def, ok := sites[a]
		if !ok {
			return fmt.Errorf("v%d is read in block %d but is not in any block", a.Id, b.Id)
		}
		if def.block == b && def.index >= i {
			return fmt.Errorf("v%d is read at position %d of block %d before it's defined at %d", a.Id, i, b.Id, def.index)
		}
		if !dom.dominates(def.block, b) {
			return fmt.Errorf("v%d is read in block %d, which its block %d doesn't dominate", a.Id, b.Id, def.block.Id)
		}
		return nil
	}

	for _, b := range f.Blocks {
		for i, v := range b.Values {
			for _, a := range v.Args {
				if err := defined(a, b, i); err != nil {
					return fmt.Errorf("operand of v%d: %w", v.Id, err)
				}
			}
		}

		// the terminator runs once every instruction has
		for _, c := range b.Control {
			if err := defined(c, b, len(b.Values)); err != nil {
				return fmt.Errorf("control of block %d: %w", b.Id, err)
			}
		}
	}

	return nil
}

// count is the number of times b appears in blocks
func count(blocks []*Block, b *Block) int {
	var n int
	for _, c := range blocks {
		if c == b {
			n += 1
		}
	}
	return n
}

func blockName(b *Block) string {
	if b == nil {
		return "no block"
	}
	return fmt.Sprintf("block %d", b.Id)
}
