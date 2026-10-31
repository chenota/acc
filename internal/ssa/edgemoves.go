package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/types"
)

type MoveKind int

const (
	MoveCopy MoveKind = iota
	MoveSwap
)

// Move is one step in handing a successor's phis their operands.
type Move struct {
	Kind MoveKind
	Dst  Location
	Src  Location
	Type *types.Type
}

// EdgeMoves returns what b has to move as it leaves for its successors, so each successor's phis find their operand
// from b in their own location. The moves are ordered to run one at a time.
func (b *Block) EdgeMoves() []Move {
	var pending []Move
	for _, s := range b.Successors {
		i := slices.Index(s.Predecessors, b)
		for _, phi := range s.Values[:s.phiIndex()] {
			src := phi.Args[i].Loc
			if phi.Loc.Kind == LocNone || phi.Loc == src {
				continue
			}
			pending = append(pending, Move{Kind: MoveCopy, Dst: phi.Loc, Src: src, Type: phi.Type})
		}
	}

	return sequentialize(pending)
}

// sequentialize orders moves meant to happen all at once so they can run one at a time.
// No two moves may share a destination.
func sequentialize(pending []Move) []Move {
	var moves []Move
	for len(pending) > 0 {
		// a move can run once nothing else still waits to read its destination
		i := slices.IndexFunc(pending, func(m Move) bool { return !readsFrom(pending, m.Dst) })
		if i >= 0 {
			moves = append(moves, pending[i])
			pending = slices.Delete(pending, i, i+1)
			continue
		}

		// every destination left is waiting to be read, so what's left are cycles, which only a swap gets around
		m := pending[0]
		pending = pending[1:]
		moves = append(moves, Move{Kind: MoveSwap, Dst: m.Dst, Src: m.Src, Type: m.Type})

		// the swap leaves what Dst held in Src
		for j := range pending {
			if pending[j].Src == m.Dst {
				pending[j].Src = m.Src
			}
		}
		// closing a cycle leaves its last move with nothing to do
		pending = slices.DeleteFunc(pending, func(m Move) bool { return m.Dst == m.Src })
	}

	return moves
}

func readsFrom(moves []Move, loc Location) bool {
	return slices.ContainsFunc(moves, func(m Move) bool { return m.Src == loc })
}
