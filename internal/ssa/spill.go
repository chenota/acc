package ssa

import (
	"maps"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/register"
)

// spill lowers register pressure using MIN algorithm.
func spill(f *Func) error {
	s := newSpiller(f)

	for b := range f.OrderedBlocks() {
		// start from what's in registers
		s.holders = s.enter(b)

		// evict and reload insert into b.Values, so walk and scan a copy taken before they do
		s.vals = slices.Clone(b.Values)
		for p, v := range s.vals {
			// reload every spilled operand of this value
			s.reloadReads(b, p, v, v.Args)

			// values whose span is over by now hand their registers back
			s.releaseEnded(b, p)

			// values that need no register, or already have a home from the abi, aren't ours to place
			if !spillable(v) {
				continue
			}

			avail := s.avail[v]
			s.makeRoom(b, p, avail, v.Args)
			s.take(v, avail)
		}

		// make sure the terminator has its control values available
		s.reloadReads(b, len(s.vals), nil, b.Control)

		s.exits[b] = s.holders
	}

	for _, v := range s.dropped {
		f.removeIfDead(v)
	}

	return nil
}

type spiller struct {
	f *Func

	// fixed before the walk starts
	uses  nextUses                 // how far past each block's end every value it leaves live is next read
	avail map[*Value]register.Mask // the registers regalloc could house each value in

	// where each value holds its register, the same spans regalloc goes by, kept current as reloads are added
	ranges liveRanges

	// what the walk has decided so far
	holders map[register.Register]*Value            // the value sitting in each occupied register
	exits   map[*Block]map[register.Register]*Value // what each block leaves in the register file
	slots   map[*Value]*Slot                        // where each evicted value was stored
	dropped []*Value                                // rematerializable values evicted without a spill slot

	// the block being walked, as it was before spilling inserted anything
	vals []*Value
}

func newSpiller(f *Func) *spiller {
	s := &spiller{
		f:      f,
		uses:   computeNextUses(f),
		ranges: computeLiveRanges(f),
		avail:  make(map[*Value]register.Mask),
		exits:  make(map[*Block]map[register.Register]*Value),
		slots:  make(map[*Value]*Slot),
	}

	for v := range f.UnorderedValues() {
		if spillable(v) {
			s.avail[v] = register.Allocatable.Without(s.ranges.blockedFor(v))
		}
	}

	return s
}

// enter is values that b's predecessors left in registers
// TODO: loops need some special handling since the header is walked before the predecessor
func (s *spiller) enter(b *Block) map[register.Register]*Value {
	var exits []map[register.Register]*Value
	for _, p := range b.Predecessors {
		if exit, ok := s.exits[p]; ok {
			exits = append(exits, exit)
		}
	}

	regs := make(map[register.Register]*Value)
	if len(exits) == 0 {
		return regs
	}

	for r, v := range exits[0] {
		// only capture values that are needed at the start of the block
		needed := s.ranges.liveOnEntry(b, v)
		// only capture a value if all b's predecesors put that value in the same register
		everywhere := !slices.ContainsFunc(exits[1:], func(exit map[register.Register]*Value) bool { return exit[r] != v })
		if needed && everywhere {
			regs[r] = v
		}
	}

	return regs
}

// reloadReads brings every spilled value in reads back into a register for the reader at position p of b
func (s *spiller) reloadReads(b *Block, p int, at *Value, reads []*Value) {
	for i, a := range reads {
		// if a isn't ours to move or already has a register we can skip
		if !spillable(a) || s.resident(a) {
			continue
		}

		m := s.scratch(b, p)
		s.makeRoom(b, p, m, reads)

		reload := s.reload(b, at, a)
		reads[i] = reload
		s.take(reload, m)

		// the reload holds its register only for the read it supports
		end := p
		if at != nil {
			end = readEnd(at, i, p)
		}
		s.ranges[b][reload] = span{Start: p, End: end}
	}
}

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

// scratch is the set of registers nothing has been pinned to across position p of b.
func (s *spiller) scratch(b *Block, p int) register.Mask {
	return register.Allocatable.Without(s.ranges.blockedAt(b, p))
}

// makeRoom evicts resident values until m has a register
func (s *spiller) makeRoom(b *Block, p int, m register.Mask, operands []*Value) {
	for m.Without(s.used()).Count() == 0 {
		victim := s.pickVictim(b, p, m, operands)
		if victim == nil {
			return // nothing left to give up
		}
		s.evict(victim)
	}
}

func (s *spiller) pickVictim(b *Block, p int, m register.Mask, operands []*Value) *Value {
	var victim *Value
	var far int
	for r := range m.Intersect(s.used()).All() {
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
	free := m.Without(s.used())
	pick := free.Intersect(register.CallerSaved)
	if pick.Count() == 0 {
		pick = free
	}

	if r, ok := pick.One(); ok {
		s.holders[r] = v
	}
}

// releaseEnded hands back the register of every resident value whose span in b is over by position p
func (s *spiller) releaseEnded(b *Block, p int) {
	for r, h := range s.holders {
		if s.ranges[b][h].End <= p {
			delete(s.holders, r)
		}
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
func (s *spiller) evict(victim *Value) {
	if victim.Rematerializable() {
		// completely drop rematerializable victims
		s.dropped = append(s.dropped, victim)
	} else if s.slots[victim] == nil {
		slot := s.f.newSlot(nil, victim.Type)
		store := s.f.insertValueAfter(victim, OpStaticStore, victim.Type, victim.Block)
		store.Args = []*Value{victim}
		store.Value = slot
		s.slots[victim] = slot
	}

	s.release(victim)
}

// reload brings evicted a back into a register just before at reads it, or at the end of b when the terminator does.
func (s *spiller) reload(b *Block, at *Value, a *Value) *Value {
	insert := func(op Op) *Value {
		if at == nil {
			return s.f.appendValue(op, a.Type, b)
		}
		return s.f.insertValueBefore(at, op, a.Type, b)
	}

	if a.Rematerializable() {
		// recomputing is cheaper than a round trip through memory
		r := insert(a.Op)
		r.Value = a.Value
		r.Offset = a.Offset
		return r
	}

	r := insert(OpStaticLoad)
	r.Value = s.slots[a]
	return r
}

// nextUse is how many instructions past p the next read of v is, and whether it is read again at all.
func (s *spiller) nextUse(b *Block, p int, v *Value) (int, bool) {
	// try to find next use in the block itself, terminator included
	for q, u := range reads(s.vals, b.Control) {
		if q > p && u == v {
			return q - p, true
		}
	}

	// if theres' a shortest-out distance: rest of block left to go + shortest-out distance of v
	if d, ok := s.uses[b][v]; ok {
		return (len(s.vals) - p) + d, true
	}

	return 0, false
}

// nextUses holds, for every block, how many instructions past its end each value it leaves live is next read.
type nextUses map[*Block]map[*Value]int

// computeNextUses finds the next-use distance at every block's end, along whichever path reaches a read soonest
func computeNextUses(f *Func) nextUses {
	// distances from a block's first instruction, which its predecessors build on
	entry := make(map[*Block]map[*Value]int)
	exit := make(nextUses)
	for _, b := range f.Blocks {
		entry[b] = make(map[*Value]int)
		exit[b] = make(map[*Value]int)
	}

	// settle a block's successors before the block itself
	order := iterutil.Reverse(f.OrderedBlocks())

	// loop until a fixpoint
	for changed := true; changed; {
		changed = false
		for b := range order {
			// a value is as close at b's end as it is at the nearest successor's start
			// TODO: weight edges that leave a loop so a use after the loop doesn't look close from inside it
			out := make(map[*Value]int)
			for _, s := range b.Successors {
				for v, d := range entry[s] {
					if cur, ok := out[v]; !ok || d < cur {
						out[v] = d
					}
				}
			}

			// anything still needed after b has to, at most, survive all of b
			in := make(map[*Value]int)
			for v, d := range out {
				in[v] = len(b.Values) + d
			}
			for p, v := range reads(b.Values, b.Control) {
				if d, ok := in[v]; !ok || p < d {
					in[v] = p
				}
			}
			for _, v := range b.Values {
				delete(in, v)
			}

			if !maps.Equal(entry[b], in) || !maps.Equal(exit[b], out) {
				changed = true
			}
			entry[b], exit[b] = in, out
		}
	}

	return exit
}
