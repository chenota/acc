package ssa

import (
	"maps"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
)

// liveness holds the values live at each block's boundaries. Each live value maps to its next-use distance.
type liveness map[*Block]blockLiveness

type blockLiveness struct {
	in  map[*Value]int // live at the block's first instruction
	out map[*Value]int // live once the block hands off to a successor
}

// computeLiveness finds what's live across every block boundary in f
func computeLiveness(f *Func) liveness {
	lv := make(liveness)
	for _, b := range f.Blocks {
		lv[b] = blockLiveness{in: make(map[*Value]int), out: make(map[*Value]int)}
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
	// add each successors' live in values to our live out set
	// TODO: weight edges that leave a loop so a use after the loop doesn't look close from inside it
	// TODO: a phi reads each operand on the edge from its predecessor, so phi operands belong in that predecessor's out
	out := make(map[*Value]int)
	for _, s := range b.Successors {
		for v, d := range lv[s].in {
			// want the shortest next-use distance for each one
			if cur, ok := out[v]; !ok || d < cur {
				out[v] = d
			}
		}
	}

	// anything still needed after b has to survive all of b
	in := make(map[*Value]int)
	for v, d := range out {
		in[v] = len(b.Values) + d
	}

	// the terminator reads control values once b's last instruction is done
	for _, c := range b.Control {
		in[c] = len(b.Values)
	}

	for i, v := range slices.Backward(b.Values) {
		// v was defined in this block, so cannot be live in
		delete(in, v)
		for _, a := range v.Args {
			// walking backwards so this continuously shortens the next-use distance of a
			in[a] = i
		}
	}

	moved := !maps.Equal(lv[b].in, in) || !maps.Equal(lv[b].out, out)
	lv[b] = blockLiveness{in: in, out: out}
	return moved
}
