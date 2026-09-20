package ssa

import (
	"iter"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/register"
)

// spill lowers register pressure using MIN algorithm.
func spill(f *Func) {
	timeline := iterutil.Enumerate(f.OrderedValues())

	s := newSpiller(f, timeline)

	for p, v := range timeline {
		// reload every spilled operand of this value
		for i, a := range v.Args {
			// if a doesn't need a register or already has one we can skip
			if !a.NeedsRegister() || s.resident(a) {
				continue
			}

			m := s.scratch(p)
			s.makeRoom(f, v, p, m)

			reload := f.insertValueBefore(v, OpStaticLoad, a.Type, v.Block)
			reload.Value = s.state(a).slot
			v.Args[i] = reload
			s.take(reload, m)
		}

		// operands with no further use are ejected
		for _, a := range v.Args {
			if _, ok := s.nextUse(a, p); !ok {
				s.release(a)
			}
		}

		if !v.NeedsRegister() {
			continue
		}

		// ignore values forced into a register by the abi
		if v.Loc.Kind != LocNone {
			s.state(v).inReg = true
			continue
		}

		avail := s.state(v).avail
		s.makeRoom(f, v, p, avail)
		s.take(v, avail)
	}
}

type spiller struct {
	values   map[*Value]*valueState
	holders  map[register.Register]*Value // the value sitting on each register of the free pool
	used     register.Mask                // the registers holders covers
	blockers []*regInterval
}

type valueState struct {
	uses  []int         // every tick the value is read at
	avail register.Mask // the registers regalloc could house it in
	slot  *Slot
	inReg bool
}

func newSpiller(f *Func, timeline iter.Seq2[int, *Value]) *spiller {
	s := &spiller{
		values:  make(map[*Value]*valueState),
		holders: make(map[register.Register]*Value),
	}

	// every position where a value is read as an operand
	for p, v := range timeline {
		for _, a := range v.Args {
			st := s.state(a)
			st.uses = append(st.uses, p)
		}
	}

	intervals := computeLiveIntervals(f)
	s.blockers = computeRegIntervals(intervals)

	for _, iv := range intervals {
		if iv.Value.Loc.Kind != LocNone || !iv.Value.NeedsRegister() {
			continue
		}

		free := register.Allocatable
		for _, b := range s.blockers {
			if overlap(b.Start, b.End, iv.Start, iv.End) {
				free = free.Remove(b.Reg)
			}
		}
		s.state(iv.Value).avail = free
	}

	return s
}

// state returns v's record, creating it for the values spill introduces itself.
func (s *spiller) state(v *Value) *valueState {
	st, ok := s.values[v]
	if !ok {
		st = &valueState{}
		s.values[v] = st
	}
	return st
}

func (s *spiller) resident(v *Value) bool {
	return s.state(v).inReg
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
func (s *spiller) makeRoom(f *Func, cur *Value, p int, m register.Mask) {
	for (m &^ s.used).Count() == 0 {
		victim := s.pickVictim(p, m, cur.Args)
		if victim == nil {
			return // nothing left to give up
		}
		s.evict(f, cur, victim)
	}
}

func (s *spiller) pickVictim(p int, m register.Mask, operands []*Value) *Value {
	var victim *Value
	var far int
	for r := range (m & s.used).All() {
		cand := s.holders[r]
		if slices.Contains(operands, cand) {
			continue
		}

		d, ok := s.nextUse(cand, p)
		if !ok {
			return cand // never read again, so nothing is cheaper to give up
		}

		if victim == nil || d > far {
			far, victim = d, cand
		}
	}
	return victim
}

// take tentatively assigns v a register out of m
func (s *spiller) take(v *Value, m register.Mask) {
	s.state(v).inReg = true

	// prefer caller-saved
	free := m &^ s.used
	pick := free & register.CallerSaved
	if pick.Count() == 0 {
		pick = free
	}

	r, ok := pick.One()
	if !ok {
		return
	}

	s.holders[r] = v
	s.used = s.used.Include(r)
}

// release marks v unreadable and hands back whatever register it was holding.
func (s *spiller) release(v *Value) {
	s.state(v).inReg = false

	for r := range s.used.All() {
		if s.holders[r] == v {
			delete(s.holders, r)
			s.used = s.used.Remove(r)
			return
		}
	}
}

// evict stores victim to its stack slot so its register can be reused.
func (s *spiller) evict(f *Func, cur *Value, victim *Value) {
	st := s.state(victim)

	if st.slot == nil {
		st.slot = f.newSlot(nil, victim.Type)
		store := f.insertValueBefore(cur, OpStaticStore, victim.Type, cur.Block)
		store.Args = []*Value{victim}
		store.Value = st.slot
	}

	s.release(victim)
}

// nextUse is the next tick v is read at, and whether it is read again at all.
func (s *spiller) nextUse(v *Value, after int) (int, bool) {
	for _, u := range s.state(v).uses {
		if u > after {
			return u, true
		}
	}
	return 0, false
}
