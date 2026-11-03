package ssa

import (
	"iter"
	"maps"

	"github.com/chenota/acc/internal/iterutil"
)

// nextUses holds the values live at each block's boundaries, along with how far away each one's next read is
type nextUses map[*Block]blockUses

type blockUses struct {
	in  map[*Value]int // live at the block's first instruction, and next read this many instructions in
	out map[*Value]int // live once the block hands off to a successor, and next read this many instructions past its end
}

// computeNextUses finds what's live across every block boundary in f, and how soon it's read along its shortest path
func computeNextUses(f *Func) nextUses {
	uses := make(nextUses)
	for _, b := range f.Blocks {
		uses[b] = blockUses{in: make(map[*Value]int), out: make(map[*Value]int)}
	}

	// heuristic: leaving a loop costs more than the longest path through f without leaving one
	loops := f.Loops()
	exitCost := 0
	for _, b := range f.Blocks {
		exitCost += len(b.Values) + 1
	}
	cost := func(b, s *Block) int { return exitCost * exits(loops, b, s) }

	// settle a block's successors before the block itself
	order := iterutil.Reverse(f.OrderedBlocks())

	for changed := true; changed; {
		changed = false
		for b := range order {
			changed = uses.update(b, cost) || changed
		}
	}

	return uses
}

// exits counts the loops an edge from b to s leaves
func exits(loops map[*Block]*Loop, b, s *Block) int {
	n := 0
	for l := loops[b]; l != nil; l = l.Outer {
		if _, ok := l.Blocks[s]; ok {
			break
		}
		n++
	}
	return n
}

// update recomputes b's boundaries from its successors, with cost added to whatever is read past each edge, and
// reports whether either one moved.
func (u nextUses) update(b *Block, cost func(b, s *Block) int) bool {
	out := make(map[*Value]int)
	for _, s := range b.Successors {
		w := cost(b, s)
		for v, d := range u[s].in {
			if cur, ok := out[v]; !ok || d+w < cur {
				out[v] = d + w
			}
		}
		for _, a := range s.PhiArgsFrom(b) {
			out[a] = 0
		}
	}

	// out values are read past all of b; b's own reads are nearer, and values b defines aren't live in
	in := make(map[*Value]int)
	for v, d := range out {
		in[v] = len(b.Values) + d
	}
	for p, v := range reads(b.Values, b.Control) {
		if d, ok := in[v]; !ok || p < d {
			in[v] = p
		}
	}
	for _, v := range b.Values {
		delete(in, v)
	}

	moved := !maps.Equal(u[b].in, in) || !maps.Equal(u[b].out, out)
	u[b] = blockUses{in: in, out: out}
	return moved
}

// reads yields each value a block's instructions read, with the position it's read at.
// A phi reads its operands at the end of its predecessors, so they aren't reads of this block.
func reads(vals []*Value, control []*Value) iter.Seq2[int, *Value] {
	return func(yield func(int, *Value) bool) {
		for p, v := range vals {
			if v.Op == OpPhi {
				continue
			}
			for _, a := range v.Args {
				if !yield(p, a) {
					return
				}
			}
		}
		for _, c := range control {
			if !yield(len(vals), c) {
				return
			}
		}
	}
}
