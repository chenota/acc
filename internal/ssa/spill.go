package ssa

import (
	"slices"

	"github.com/chenota/acc/internal/register"
)

// spill lowers register pressure using MIN algorithm.
func spill(f *Func) {
	s := newSpiller(f, computeLiveness(f))

	// track instructions count across the whole function
	// TODO: switch blockers to block-relative positions once regalloc builds per-block ranges too
	tick := 0
	for b := range f.OrderedBlocks() {
		// evict and reload insert into b.Values, so walk and scan a copy taken before they do
		s.vals = slices.Clone(b.Values)
		for p, v := range s.vals {
			// reload every spilled operand of this value
			for i, a := range v.Args {
				// if a isn't ours to move or already has a register we can skip
				if !spillable(a) || s.resident(a) {
					continue
				}

				m := s.scratch(tick + p)
				s.makeRoom(b, v, p, m)

				reload := s.reload(v, a)
				v.Args[i] = reload
				s.take(reload, m)
			}

			// operands with no further use are ejected
			for _, a := range v.Args {
				if _, ok := s.nextUse(b, p, a); !ok {
					s.release(a)
				}
			}

			// values that need no register, or already have a home from the abi, aren't ours to place
			if !spillable(v) {
				continue
			}

			avail := s.avail[v]
			s.makeRoom(b, v, p, avail)
			s.take(v, avail)
		}

		tick += len(s.vals)
	}

	for _, v := range s.dropped {
		f.removeIfDead(v)
	}
}

type spiller struct {
	f *Func

	// fixed before the walk starts
	live     liveness
	blockers []*regInterval
	avail    map[*Value]register.Mask // the registers regalloc could house each value in

	// what the walk has decided so far
	holders map[register.Register]*Value // the value sitting in each occupied register
	slots   map[*Value]*Slot             // where each evicted value was stored
	dropped []*Value                     // rematerializable values evicted without a spill slot

	// the block being walked, as it was before spilling inserted anything
	vals []*Value
}

func newSpiller(f *Func, live liveness) *spiller {
	s := &spiller{
		f:       f,
		live:    live,
		avail:   make(map[*Value]register.Mask),
		holders: make(map[register.Register]*Value),
		slots:   make(map[*Value]*Slot),
	}

	intervals := computeLiveIntervals(f)
	s.blockers = computeRegIntervals(intervals)

	for _, iv := range intervals {
		if !spillable(iv.Value) {
			continue
		}

		free := register.Allocatable
		for _, b := range s.blockers {
			if overlap(b.Start, b.End, iv.Start, iv.End) {
				free = free.Remove(b.Reg)
			}
		}
		s.avail[iv.Value] = free
	}

	return s
}

// spillable reports whether v is spill's to place: it needs a register, and the abi hasn't already given it a home.
func spillable(v *Value) bool {
	return v.NeedsRegister() && v.Loc.Kind == LocNone
}

// resident reports whether v is sitting in a register.
func (s *spiller) resident(v *Value) bool {
	for _, h := range s.holders {
		if h == v {
			return true
		}
	}
	return false
}

// used is the set of registers holding a value.
func (s *spiller) used() register.Mask {
	var m register.Mask
	for r := range s.holders {
		m = m.Include(r)
	}
	return m
}

// scratch is the set of registers nothing has been pinned to across tick p.
func (s *spiller) scratch(p int) register.Mask {
	free := register.Allocatable
	for _, b := range s.blockers {
		if overlap(b.Start, b.End, p, p+1) {
			free = free.Remove(b.Reg)
		}
	}
	return free
}

// makeRoom evicts resident values until m has a register to spare.
func (s *spiller) makeRoom(b *Block, cur *Value, p int, m register.Mask) {
	for (m &^ s.used()).Count() == 0 {
		victim := s.pickVictim(b, p, m, cur.Args)
		if victim == nil {
			return // nothing left to give up
		}
		s.evict(cur, victim)
	}
}

func (s *spiller) pickVictim(b *Block, p int, m register.Mask, operands []*Value) *Value {
	var victim *Value
	var far int
	for r := range (m & s.used()).All() {
		cand := s.holders[r]
		if slices.Contains(operands, cand) {
			continue
		}

		d, ok := s.nextUse(b, p, cand)
		if !ok {
			return cand // never read again, so nothing is cheaper to give up
		}

		if victim == nil {
			far, victim = d, cand
			continue
		}

		// prefer rematerializable candidates
		cheaper := cand.Rematerializable() && !victim.Rematerializable()
		same := cand.Rematerializable() == victim.Rematerializable()
		if cheaper || (same && d > far) {
			far, victim = d, cand
		}
	}
	return victim
}

// take tentatively assigns v a register out of m
func (s *spiller) take(v *Value, m register.Mask) {
	// prefer caller-saved
	free := m &^ s.used()
	pick := free & register.CallerSaved
	if pick.Count() == 0 {
		pick = free
	}

	if r, ok := pick.One(); ok {
		s.holders[r] = v
	}
}

// release hands back whatever register v was holding.
func (s *spiller) release(v *Value) {
	for r, h := range s.holders {
		if h == v {
			delete(s.holders, r)
			return
		}
	}
}

// evict frees victim's register for reuse
func (s *spiller) evict(cur *Value, victim *Value) {
	if victim.Rematerializable() {
		// completely drop rematerializable victims
		s.dropped = append(s.dropped, victim)
	} else if s.slots[victim] == nil {
		slot := s.f.newSlot(nil, victim.Type)
		store := s.f.insertValueBefore(cur, OpStaticStore, victim.Type, cur.Block)
		store.Args = []*Value{victim}
		store.Value = slot
		s.slots[victim] = slot
	}

	s.release(victim)
}

// reload brings evicted a back into a register just before v reads it.
func (s *spiller) reload(v *Value, a *Value) *Value {
	if a.Rematerializable() {
		// recomputing is cheaper than a round trip through memory
		r := s.f.insertValueBefore(v, a.Op, a.Type, v.Block)
		r.Value = a.Value
		r.Offset = a.Offset
		return r
	}

	r := s.f.insertValueBefore(v, OpStaticLoad, a.Type, v.Block)
	r.Value = s.slots[a]
	return r
}

// nextUse is how many instructions past p the next read of v is, and whether it is read again at all.
func (s *spiller) nextUse(b *Block, p int, v *Value) (int, bool) {
	// try to find next use in the block itself
	for i, u := range s.vals[p+1:] {
		if slices.Contains(u.Args, v) {
			return i + 1, true
		}
	}

	// check if it's a control value
	if slices.Contains(b.Control, v) {
		return len(s.vals) - p, true
	}

	// if theres' a shortest-out distance: rest of block left to go + shortest-out distance of v
	if d, ok := s.live[b].out[v]; ok {
		return (len(s.vals) - p) + d, true
	}

	return 0, false
}
