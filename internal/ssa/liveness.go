package ssa

import (
	"iter"
	"maps"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
)

// liveness holds the values live at each block's boundaries
type liveness map[*Block]blockLiveness

type blockLiveness struct {
	in  map[*Value]struct{} // live at the block's first instruction
	out map[*Value]struct{} // live once the block hands off to a successor
}

// computeLiveness finds what's live across every block boundary in f
func computeLiveness(f *Func) liveness {
	lv := make(liveness)
	for _, b := range f.Blocks {
		lv[b] = blockLiveness{in: make(map[*Value]struct{}), out: make(map[*Value]struct{})}
	}

	// settle a block's successors before the block itself
	order := iterutil.Reverse(f.OrderedBlocks())

	// loop until a fixpoint
	for changed := true; changed; {
		changed = false
		for b := range order {
			changed = lv.update(b) || changed
		}
	}

	return lv
}

// update recomputes b's boundaries from its successors and reports whether either one moved.
func (lv liveness) update(b *Block) bool {
	// anything a successor needs is needed once b is done
	out := make(map[*Value]struct{})
	for _, s := range b.Successors {
		maps.Copy(out, lv[s].in)

		// copy each predecessor's phis that this block outputs to its out set
		for _, p := range b.Successors {
			i := slices.Index(p.Predecessors, b)
			for _, v := range p.Values[:p.phiIndex()] {
				out[v.Args[i]] = struct{}{}
			}
		}
	}

	// so is anything b reads, unless b defines it first
	in := maps.Clone(out)
	// want to skip phi args in reads to prevent erraneous values getting copied into successors out sets
	// NOTE: this omits phi args from the in set of this block which may cause issues in the future but works for now
	for _, v := range reads(b.Values[b.phiIndex():], b.Control) {
		in[v] = struct{}{}
	}
	for _, v := range b.Values {
		delete(in, v)
	}

	moved := !maps.Equal(lv[b].in, in) || !maps.Equal(lv[b].out, out)
	lv[b] = blockLiveness{in: in, out: out}
	return moved
}

// reads yields each value a block's instructions read, with the position it's read at.
func reads(vals []*Value, control []*Value) iter.Seq2[int, *Value] {
	return func(yield func(int, *Value) bool) {
		for p, v := range vals {
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
