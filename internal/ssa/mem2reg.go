package ssa

import (
	"errors"
	"fmt"
	"iter"
)

var errNeedsMerge = errors.New("load needs values from different paths merged")

// mem2reg replaces loads and stores of promotable slots with SSA values.
// Follows the Braun on-demand lookup algorithm
func mem2reg(f *Func) error {
	for slot := range promotableSlots(f) {
		loaded, err := resolveLoads(f, slot)
		if errors.Is(err, errNeedsMerge) {
			// TODO: merge the values with a phi
			continue
		}
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

// resolveLoads maps each of slot's loads to the value it reads
func resolveLoads(f *Func, slot *Slot) (map[*Value]*Value, error) {
	// each block's current value for each leaf, keyed by offset
	defs := make(map[*Block]map[int]*Value)
	for _, b := range f.Blocks {
		defs[b] = make(map[int]*Value)
	}

	loaded := make(map[*Value]*Value)
	for b := range f.OrderedBlocks() {
		for _, v := range b.Values {
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
				defs[b][v.Offset] = val
			case OpStaticLoad:
				val, err := readLeaf(defs, b, v.Offset)
				if err != nil {
					return nil, err
				}
				loaded[v] = val
			}
		}
	}

	return loaded, nil
}

// readLeaf returns the leaf's value at the walk's current point in b, or errNeedsMerge if b's predecessors disagree
func readLeaf(defs map[*Block]map[int]*Value, b *Block, offset int) (*Value, error) {
	if v, ok := defs[b][offset]; ok {
		return v, nil
	}

	if len(b.Predecessors) == 0 {
		return nil, errors.New("slot read before anything is stored to it")
	}

	// predecessors come first in rpo, so the value each one ends on is already settled
	// TODO: loops break this since a back edge comes from a block the walk hasn't reached
	var v *Value
	for _, pred := range b.Predecessors {
		pv, err := readLeaf(defs, pred, offset)
		if err != nil {
			return nil, err
		}
		if v != nil && pv != v {
			return nil, errNeedsMerge
		}
		v = pv
	}

	defs[b][offset] = v
	return v, nil
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
