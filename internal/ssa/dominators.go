package ssa

import "slices"

// domTree maps each reachable block to its immediate dominator
type domTree map[*Block]*Block

// computeDominators finds every reachable block's immediate dominator
// Uses the Cooper-Harvey-Kennedy algorithm
func computeDominators(f *Func) domTree {
	order := slices.Collect(f.OrderedBlocks())

	// a block's rpo number orders it after everything that dominates it
	rpo := make(map[*Block]int, len(order))
	for i, b := range order {
		rpo[b] = i
	}

	// entry is its own immediate dominator since no precdecessors
	idom := domTree{f.Entry: f.Entry}

	// find nearest common denominator of two blocks
	intersect := func(a, b *Block) *Block {
		for a != b {
			for rpo[a] > rpo[b] {
				a = idom[a]
			}
			for rpo[b] > rpo[a] {
				b = idom[b]
			}
		}
		return a
	}

	// loop until a fixpoint
	for changed := true; changed; {
		changed = false
		// skip entry block
		for _, b := range order[1:] {
			var next *Block
			for _, p := range b.Predecessors {
				// skip predecessors without an immediate dominator
				if _, ok := idom[p]; !ok {
					continue
				}
				if next == nil {
					next = p
					continue
				}
				next = intersect(p, next)
			}

			if idom[b] != next {
				idom[b] = next
				changed = true
			}
		}
	}

	return idom
}

// nearestCommon returns the closest block dominating both a and b
func (d domTree) nearestCommon(a, b *Block) *Block {
	for !d.dominates(a, b) {
		a = d[a]
	}
	return a
}

// dominates reports whether every path from the entry to b passes through a
func (d domTree) dominates(a, b *Block) bool {
	for {
		if a == b {
			return true
		}
		up, ok := d[b]
		if !ok || up == b {
			// left the tree without passing a
			return false
		}
		b = up
	}
}
