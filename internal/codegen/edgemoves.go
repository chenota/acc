package codegen

import (
	"slices"

	"github.com/chenota/acc/internal/ssa"
)

type moveKind int

const (
	moveCopy moveKind = iota
	moveSwap
)

type move struct {
	kind moveKind
	dst  ssa.Location
	src  ssa.Location
	size int
}

// sequentialize orders moves meant to happen all at once so they can run one at a time.
func sequentialize(pending []move) []move {
	var moves []move
	for len(pending) > 0 {
		// a move can run once nothing else still waits to read its destination
		i := slices.IndexFunc(pending, func(m move) bool { return !readsFrom(pending, m.dst) })
		if i >= 0 {
			moves = append(moves, pending[i])
			pending = slices.Delete(pending, i, i+1)
			continue
		}

		// every destination left is waiting to be read, so what's left are cycles, which only a swap gets around
		m := pending[0]
		pending = pending[1:]
		moves = append(moves, move{kind: moveSwap, dst: m.dst, src: m.src, size: m.size})

		// the swap leaves what dst held in src
		for j := range pending {
			if pending[j].src == m.dst {
				pending[j].src = m.src
			}
		}
		// closing a cycle leaves its last move with nothing to do
		pending = slices.DeleteFunc(pending, func(m move) bool { return m.dst == m.src })
	}

	return moves
}

func readsFrom(moves []move, loc ssa.Location) bool {
	return slices.ContainsFunc(moves, func(m move) bool { return m.src == loc })
}
