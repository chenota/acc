package ssa

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/chenota/acc/internal/types"
)

// mem2reg replaces loads and stores of promotable slots with SSA values.
// Based on Braun et al., "Simple and Efficient Construction of Static Single Assignment Form".
func mem2reg(f *Func) error {
	for slot := range promotableSlots(f) {
		loaded, err := resolveLoads(f, slot)
		if err != nil {
			return fmt.Errorf("mem2reg: %s: %w", f.Name(), err)
		}

		for v := range f.SlotValues(slot) {
			if v.Op == OpStaticLoad {
				f.redirectUses(v, loaded[v])
			}
			f.removeValue(v)
		}
	}

	return nil
}

// promoter is one slot's progress through mem2reg
type promoter struct {
	f *Func

	defs       map[*Block]map[int]*Value // each block's current value for each leaf, keyed by offset
	incomplete map[*Block]map[int]*Value // phis in unsealed blocks, which get their operands once the block is sealed
	filled     map[*Block]bool           // blocks the walk is done with
	sealed     map[*Block]bool           // blocks whose predecessors are all filled
}

// resolveLoads maps each of slot's loads to the value it reads
func resolveLoads(f *Func, slot *Slot) (map[*Value]*Value, error) {
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

	loaded := make(map[*Value]*Value)
	for b := range f.OrderedBlocks() {
		for _, v := range slices.Clone(b.Values) {
			if v.Slot() != slot {
				continue
			}
			switch v.Op {
			case OpStaticStore:
				val := v.Args[0]
				// see through a load of the slot to the value it reads
				if r, ok := loaded[val]; ok {
					val = r
				}
				p.defs[b][v.Offset] = val
			case OpStaticLoad:
				val, err := p.readLeaf(b, v.Offset, v.Type)
				if err != nil {
					return nil, err
				}
				loaded[v] = val
			}
		}

		p.filled[b] = true
		for _, s := range b.Successors {
			if err := p.trySeal(s); err != nil {
				return nil, err
			}
		}
	}

	return loaded, nil
}

// readLeaf returns the leaf's value at the walk's current point in b
func (p *promoter) readLeaf(b *Block, offset int, t *types.Type) (*Value, error) {
	if v, ok := p.defs[b][offset]; ok {
		return v, nil
	}

	var v *Value
	var err error
	switch {
	case !p.sealed[b]:
		// a predecessor the walk hasn't reached may still store to the slot
		v = p.f.insertValueFront(OpPhi, t, b)
		p.incomplete[b][offset] = v
	case len(b.Predecessors) == 0:
		return nil, errors.New("slot read before anything is stored to it")
	case len(b.Predecessors) == 1:
		v, err = p.readLeaf(b.Predecessors[0], offset, t)
	default:
		// record the phi first so a cycle back to b ends at it
		phi := p.f.insertValueFront(OpPhi, t, b)
		p.defs[b][offset] = phi
		if err = p.addPhiOperands(b, offset, phi); err != nil {
			break
		}

		v = phi
		if !slices.ContainsFunc(phi.Args[1:], func(a *Value) bool { return a != phi.Args[0] }) {
			// every predecessor leaves the same value, so there's nothing to merge
			v = phi.Args[0]
			p.f.removeValue(phi)
		}
	}
	if err != nil {
		return nil, err
	}

	p.defs[b][offset] = v
	return v, nil
}

// addPhiOperands gives phi the value each of b's predecessors leaves in the leaf
func (p *promoter) addPhiOperands(b *Block, offset int, phi *Value) error {
	for _, pred := range b.Predecessors {
		v, err := p.readLeaf(pred, offset, phi.Type)
		if err != nil {
			return err
		}
		phi.Args = append(phi.Args, v)
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

			// anything other than a static load and store of a slot pins to memory
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
