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

	users    map[*Value][]*Value // the phis that take each value as an operand
	replaced map[*Value]*Value   // what stands in for each phi removed as trivial
}

// resolveLoads maps each of slot's loads to the value it reads
func resolveLoads(f *Func, slot *Slot) (map[*Value]*Value, error) {
	p := &promoter{
		f:          f,
		defs:       make(map[*Block]map[int]*Value),
		incomplete: make(map[*Block]map[int]*Value),
		filled:     make(map[*Block]bool),
		sealed:     make(map[*Block]bool),
		users:      make(map[*Value][]*Value),
		replaced:   make(map[*Value]*Value),
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

	for ld, v := range loaded {
		loaded[ld] = p.resolve(v)
	}
	return loaded, nil
}

// readLeaf returns the leaf's value at the walk's current point in b
func (p *promoter) readLeaf(b *Block, offset int, t *types.Type) (*Value, error) {
	if v, ok := p.defs[b][offset]; ok {
		return p.resolve(v), nil
	}

	var v *Value
	var err error
	switch {
	case !p.sealed[b]:
		// a predecessor the walk hasn't reached may still store to the slot
		v = p.f.insertValueFront(OpPhi, t, b)
		p.incomplete[b][offset] = v
	case len(b.Predecessors) == 0:
		return nil, errReadBeforeStore
	case len(b.Predecessors) == 1:
		v, err = p.readLeaf(b.Predecessors[0], offset, t)
	default:
		// record the phi first so a cycle back to b ends at it
		phi := p.f.insertValueFront(OpPhi, t, b)
		p.defs[b][offset] = phi
		v, err = p.addPhiOperands(b, offset, phi)
	}
	if err != nil {
		return nil, err
	}

	p.defs[b][offset] = v
	return v, nil
}

// addPhiOperands gives phi the value each of b's predecessors leaves in the leaf, and returns what stands in for phi
func (p *promoter) addPhiOperands(b *Block, offset int, phi *Value) (*Value, error) {
	for _, pred := range b.Predecessors {
		v, err := p.readLeaf(pred, offset, phi.Type)
		if err != nil {
			return nil, err
		}
		phi.Args = append(phi.Args, v)
		p.users[v] = append(p.users[v], phi)
	}
	return p.tryRemoveTrivialPhi(phi)
}

// tryRemoveTrivialPhi replaces phi with the one value it merges besides itself, if that's all it merges,
// and returns what stands in for phi.
func (p *promoter) tryRemoveTrivialPhi(phi *Value) (*Value, error) {
	var same *Value
	for _, a := range phi.Args {
		if a == same || a == phi {
			continue
		}
		if same != nil {
			return phi, nil
		}
		same = a
	}
	// no value reaches phi but itself
	if same == nil {
		return nil, errReadBeforeStore
	}

	p.replaced[phi] = same
	p.f.removeValue(phi)

	users := slices.DeleteFunc(p.users[phi], func(u *Value) bool { return u == phi || p.removed(u) })
	delete(p.users, phi)
	for _, u := range users {
		for i := range u.Args {
			if u.Args[i] == phi {
				u.Args[i] = same
			}
		}
		p.users[same] = append(p.users[same], u)
	}

	// a phi that took phi as an operand may now merge only one value too
	for _, u := range users {
		if p.removed(u) {
			continue
		}
		if _, err := p.tryRemoveTrivialPhi(u); err != nil {
			return nil, err
		}
	}

	return p.resolve(same), nil
}

func (p *promoter) removed(phi *Value) bool {
	_, ok := p.replaced[phi]
	return ok
}

// resolve follows v through the trivial phis removed in its place
func (p *promoter) resolve(v *Value) *Value {
	for {
		r, ok := p.replaced[v]
		if !ok {
			return v
		}
		v = r
	}
}

// trySeal seals b once all its predecessors are filled, completing the phis that were waiting on them
func (p *promoter) trySeal(b *Block) error {
	if p.sealed[b] || slices.ContainsFunc(b.Predecessors, func(pred *Block) bool { return !p.filled[pred] }) {
		return nil
	}

	for _, offset := range slices.Sorted(maps.Keys(p.incomplete[b])) {
		if _, err := p.addPhiOperands(b, offset, p.incomplete[b][offset]); err != nil {
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
