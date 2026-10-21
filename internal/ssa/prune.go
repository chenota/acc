package ssa

import "slices"

// pruneUnreachable drops the blocks control can never reach
func pruneUnreachable(f *Func) {
	reachable := make(map[*Block]struct{})
	for b := range f.OrderedBlocks() {
		reachable[b] = struct{}{}
	}

	dead := func(b *Block) bool {
		_, ok := reachable[b]
		return !ok
	}

	f.Blocks = slices.DeleteFunc(f.Blocks, dead)

	// only predecessors can still name a dead block
	for _, b := range f.Blocks {
		b.Predecessors = slices.DeleteFunc(b.Predecessors, dead)
	}
}
