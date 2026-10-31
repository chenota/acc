package ssa

import (
	"errors"
	"fmt"
	"iter"
	"slices"

	"github.com/chenota/acc/internal/types"
)

// mem2reg replaces loads and stores of promotable slots with SSA values.
// Follows the Braun on-demand lookup algorithm
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

// resolveLoads maps each of slot's loads to the value it reads
func resolveLoads(f *Func, slot *Slot) (map[*Value]*Value, error) {
	// each block's current value for each leaf, keyed by offset
	defs := make(map[*Block]map[int]*Value)
	for _, b := range f.Blocks {
		defs[b] = make(map[int]*Value)
	}

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
				defs[b][v.Offset] = val
			case OpStaticLoad:
				val, err := readLeaf(f, defs, b, v.Offset, v.Type)
				if err != nil {
					return nil, err
				}
				loaded[v] = val
			}
		}
	}

	return loaded, nil
}

// readLeaf returns the leaf's value at the walk's current point in b
func readLeaf(f *Func, defs map[*Block]map[int]*Value, b *Block, offset int, t *types.Type) (*Value, error) {
	if v, ok := defs[b][offset]; ok {
		return v, nil
	}

	if len(b.Predecessors) == 0 {
		return nil, errors.New("slot read before anything is stored to it")
	} else if len(b.Predecessors) == 1 {
		// only one predecessor, read leaf off that directly
		v, err := readLeaf(f, defs, b.Predecessors[0], offset, t)
		if err != nil {
			return nil, err
		}
		defs[b][offset] = v
		return v, nil
	}

	// record phi first to prevent infinite loop if anything loops back to b
	phi := f.insertValueFront(OpPhi, t, b)
	defs[b][offset] = phi
	for _, predecessor := range b.Predecessors {
		predecessorValue, err := readLeaf(f, defs, predecessor, offset, t)
		if err != nil {
			return nil, err
		}
		phi.Args = append(phi.Args, predecessorValue)
	}

	var v *Value
	if !slices.ContainsFunc(phi.Args[1:], func(v *Value) bool { return v != phi.Args[0] }) {
		// all phi values are the same, no phi needed
		v = phi.Args[0]
		f.removeValue(phi)
	} else {
		// phi args differ, phi is necessary
		v = phi
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
