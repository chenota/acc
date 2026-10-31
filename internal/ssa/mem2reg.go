package ssa

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/chenota/acc/internal/types"
)

var errReadBeforeStore = errors.New("slot read before anything is stored to it")

// mem2reg replaces loads and stores of promotable slots with SSA values.
// Based on Braun et al., "Simple and Efficient Construction of Static Single Assignment Form".
func mem2reg(f *Func) error {
	for slot := range promotableSlots(f) {
		if err := promote(f, slot); err != nil {
			return fmt.Errorf("mem2reg: %s: %w", f.Name(), err)
		}

		for v := range f.SlotValues(slot) {
			f.removeValue(v)
		}
	}

	return nil
}

type promoter struct {
	f *Func

	defs       map[*Block]map[int]*Value // each block's current value for each leaf, keyed by offset
	incomplete map[*Block]map[int]*Value // phis in unsealed blocks, which get their operands once the block is sealed
	filled     map[*Block]bool           // blocks the walk is done with
	sealed     map[*Block]bool           // blocks whose predecessors are all filled
}

// promote points the uses of each of slot's loads at the value it reads
func promote(f *Func, slot *Slot) error {
	p := &promoter{
		f:          f,
		defs:       make(map[*Block]map[int]*Value),
		incomplete: make(map[*Block]map[int]*Value),
		filled:     make(map[*Block]bool),
		sealed:     make(map[*Block]bool),
	}
	for _, b := range f.Blocks {
		p.defs[b] = make(map[int]*Value)
		p.incomplete[b] = make(map[int]*Value)
	}

	// the entry has no predecessors to wait on, and every other block is sealed once its last one is filled
	p.sealed[f.Entry] = true

	for b := range f.OrderedBlocks() {
		for _, v := range slices.Clone(b.Values) {
			if v.Slot() != slot {
				continue
			}
			switch v.Op {
			case OpStaticStore:
				// a store of a load of the slot already reads what the load did, since the load comes first
				p.defs[b][v.Offset] = v.Args[0]
			case OpStaticLoad:
				val, err := p.readLeaf(b, v.Offset, v.Type)
				if err != nil {
					return err
				}
				f.redirectUses(v, val)
			}
		}

		p.filled[b] = true
		for _, s := range b.Successors {
			if err := p.trySeal(s); err != nil {
				return err
			}
		}
	}

	return nil
}

// readLeaf returns the leaf's value at the walk's current point in b
func (p *promoter) readLeaf(b *Block, offset int, t *types.Type) (*Value, error) {
	if v, ok := p.defs[b][offset]; ok {
		return v, nil
	}

	switch {
	case !p.sealed[b]:
		// a predecessor the walk hasn't reached may still store to the slot
		phi := p.f.insertValueFront(OpPhi, t, b)
		p.incomplete[b][offset] = phi
		p.defs[b][offset] = phi
	case len(b.Predecessors) == 0:
		return nil, errReadBeforeStore
	case len(b.Predecessors) == 1:
		v, err := p.readLeaf(b.Predecessors[0], offset, t)
		if err != nil {
			return nil, err
		}
		p.defs[b][offset] = v
	default:
		// record the phi first so a cycle back to b ends at it
		phi := p.f.insertValueFront(OpPhi, t, b)
		p.defs[b][offset] = phi
		if err := p.addPhiOperands(b, offset, phi); err != nil {
			return nil, err
		}
	}

	// a phi made above may have been removed as trivial since, and defs follows whatever replaced it
	return p.defs[b][offset], nil
}

// addPhiOperands gives phi the value each of b's predecessors leaves in the leaf
func (p *promoter) addPhiOperands(b *Block, offset int, phi *Value) error {
	for _, pred := range b.Predecessors {
		v, err := p.readLeaf(pred, offset, phi.Type)
		if err != nil {
			return err
		}
		phi.addArg(v)
	}
	return p.tryRemoveTrivialPhi(phi)
}

// tryRemoveTrivialPhi replaces phi with the one value it merges besides itself, if that's all it merges
func (p *promoter) tryRemoveTrivialPhi(phi *Value) error {
	var same *Value
	for _, a := range phi.Args {
		if a == same || a == phi {
			continue
		}
		if same != nil {
			return nil
		}
		same = a
	}
	// no value reaches phi but itself
	if same == nil {
		return errReadBeforeStore
	}

	// removing phi lets go of its operands, so it no longer counts among its own users
	p.f.removeValue(phi)
	users := slices.Clone(phi.users)
	p.f.redirectUses(phi, same)
	for _, defs := range p.defs {
		for offset, v := range defs {
			if v == phi {
				defs[offset] = same
			}
		}
	}

	// a phi that took phi as an operand may now merge only one value too
	for _, u := range users {
		if u.Op != OpPhi || u.Block.indexOf(u) < 0 {
			continue
		}
		if err := p.tryRemoveTrivialPhi(u); err != nil {
			return err
		}
	}
	return nil
}

// trySeal seals b once all its predecessors are filled, completing the phis that were waiting on them
func (p *promoter) trySeal(b *Block) error {
	if p.sealed[b] || slices.ContainsFunc(b.Predecessors, func(pred *Block) bool { return !p.filled[pred] }) {
		return nil
	}

	for _, offset := range slices.Sorted(maps.Keys(p.incomplete[b])) {
		if err := p.addPhiOperands(b, offset, p.incomplete[b][offset]); err != nil {
			return err
		}
	}
	p.sealed[b] = true
	return nil
}

func promotableSlots(f *Func) iter.Seq[*Slot] {
	return func(yield func(*Slot) bool) {
		pinned := make(map[*Slot]struct{})

		for v := range f.UnorderedValues() {
			s := v.Slot()
			if s == nil {
				continue
			}

			if v.Op != OpStaticLoad && v.Op != OpStaticStore {
				pinned[s] = struct{}{}
			}
		}

		for _, s := range f.Slots {
			if _, ok := pinned[s]; ok {
				continue
			}
			if !yield(s) {
				break
			}
		}
	}
}
