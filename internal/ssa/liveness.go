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

// computeNextUses finds what's live across every block boundary in f, and how soon it's read along whichever path
// reaches a read first
func computeNextUses(f *Func) nextUses {
	uses := make(nextUses)
	for _, b := range f.Blocks {
		uses[b] = blockUses{in: make(map[*Value]int), out: make(map[*Value]int)}
	}

	// settle a block's successors before the block itself
	order := iterutil.Reverse(f.OrderedBlocks())

	// loop until a fixpoint
	for changed := true; changed; {
		changed = false
		for b := range order {
			changed = uses.update(b) || changed
		}
	}

	return uses
}

// update recomputes b's boundaries from its successors and reports whether either one moved.
func (u nextUses) update(b *Block) bool {
	// TODO: weight edges that leave a loop so a use after the loop doesn't look close from inside it
	out := make(map[*Value]int)
	for _, s := range b.Successors {
		// copy successor in sets to this block's out set
		for v, d := range u[s].in {
			if cur, ok := out[v]; !ok || d < cur {
				out[v] = d
			}
		}
		// copy phi arguments going to each successor to this block's out set
		for _, a := range s.PhiArgsFrom(b) {
			out[a] = 0
		}
	}

	// so is anything b reads, unless b defines it first, and anything only needed after b has to survive all of it
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
