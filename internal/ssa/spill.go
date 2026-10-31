package ssa

import (
	"cmp"
	"maps"
	"math"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/register"
)

// spill lowers register pressure using MIN algorithm.
func spill(f *Func) error {
	s := newSpiller(f)

	for b := range f.OrderedBlocks() {
		s.holders = s.enter(b)

		// evict and reload insert into b.Values, so walk and scan a copy taken before they do
		s.vals = slices.Clone(b.Values)
		phis := s.vals[:b.phiIndex()]
		for p, v := range s.vals {
			// phis take their registers on the way into b, and their operands are read at the end of each predecessor
			if p < len(phis) {
				s.takePhi(b, p, v, phis)
				continue
			}

			s.reloadReads(b, p, v, v.Args)

			s.releaseEnded(b, p)

			if !spillable(v) {
				continue
			}

			avail := s.avail[v]
			s.makeRoom(b, p, avail, v.Args)
			s.take(v, avail)
		}

		s.storeMemoryPhiArgs(b)
		s.reloadPhiArgs(b)

		s.reloadReads(b, len(s.vals), nil, b.Control)

		s.exits[b] = s.holders
	}

	for _, v := range s.dropped {
		f.removeIfDead(v)
	}

	// every read of a memory phi was reloaded from its slot, so the phi itself has nothing left to do
	for phi := range s.memPhis {
		f.removeValue(phi)
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
	holders map[register.Register]*Value
	exits   map[*Block]map[register.Register]*Value
	slots   map[*Value]*Slot
	dropped []*Value         // rematerializable values evicted without a spill slot
	memPhis map[*Value]*Slot // phis that wait in a slot instead of a register, and that slot

	// the block being walked, as it was before spilling inserted anything
	vals []*Value
}

func newSpiller(f *Func) *spiller {
	s := &spiller{
		f:       f,
		uses:    computeNextUses(f),
		ranges:  computeLiveRanges(f),
		avail:   make(map[*Value]register.Mask),
		exits:   make(map[*Block]map[register.Register]*Value),
		slots:   make(map[*Value]*Slot),
		memPhis: make(map[*Value]*Slot),
	}

	for v := range f.UnorderedValues() {
		if spillable(v) {
			s.avail[v] = register.Allocatable.Without(s.ranges.blockedFor(v))
		}
	}

	// predecessors are walked before the phis they feed, so they need to know up front which phis wait in memory
	s.pickMemoryPhis()

	return s
}

// pickMemoryPhis decides which phis wait in a slot instead of a register
func (s *spiller) pickMemoryPhis() {
	for _, b := range s.f.Blocks {
		if b.phiIndex() == 0 {
			continue
		}

		// everything that wants a register as b starts, the nearest read first
		var live []*Value
		for v, sp := range s.ranges[b] {
			if sp.Start < 0 && spillable(v) {
				live = append(live, v)
			}
		}
		slices.SortFunc(live, func(u, v *Value) int {
			return cmp.Or(cmp.Compare(s.entryUse(b, u), s.entryUse(b, v)), cmp.Compare(u.Id, v.Id))
		})

		// hand out registers the way take would
		free := s.scratch(b, -1)
		for _, v := range live {
			if r, ok := preferCallerSaved(s.avail[v].Intersect(free)).One(); ok {
				free = free.Remove(r)
				continue
			}

			if v.Op == OpPhi && v.Block == b {
				slot := s.f.newSlot(nil, v.Type)
				s.memPhis[v] = slot
				// a memory phi is never in a register, so every read of it reloads from the slot like any spilled value
				s.slots[v] = slot
			}
		}
	}
}

// entryUse is how far past the start of b v is next read, or math.MaxInt if it never is
func (s *spiller) entryUse(b *Block, v *Value) int {
	// b's phis read their operands in the predecessors, so reads start past them
	n := b.phiIndex()
	for p, u := range reads(b.Values[n:], b.Control) {
		if u == v {
			return n + p
		}
	}
	if d, ok := s.uses[b][v]; ok {
		return len(b.Values) + d
	}
	return math.MaxInt
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
		needed := s.ranges.liveOnEntry(b, v)
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

// takePhi gives phi its register as b starts, unless it waits in memory
func (s *spiller) takePhi(b *Block, p int, phi *Value, phis []*Value) {
	if _, ok := s.memPhis[phi]; ok || !spillable(phi) {
		return
	}

	// every phi takes its register on the way in, so giving one up wouldn't make room for another
	avail := s.avail[phi]
	s.makeRoom(b, p, avail, phis)
	s.take(phi, avail)
}

// storeMemoryPhiArgs writes each operand b hands a successor's memory phi into that phi's slot as b ends
func (s *spiller) storeMemoryPhiArgs(b *Block) {
	for _, succ := range b.Successors {
		i := slices.Index(succ.Predecessors, b)
		for _, phi := range succ.Values[:succ.phiIndex()] {
			slot, ok := s.memPhis[phi]
			if !ok {
				continue
			}

			store := s.f.appendValue(OpStaticStore, phi.Type, b)
			store.Args = []*Value{phi.Args[i]}
			store.Value = slot

			s.reloadReads(b, len(s.vals), store, store.Args)
			// a reload made just for the store gives its register straight back
			if store.Args[0] != phi.Args[i] {
				s.release(store.Args[0])
			}
		}
	}
}

// reloadPhiArgs brings back every spilled operand b hands to its successors' register phis, which read them as b ends
func (s *spiller) reloadPhiArgs(b *Block) {
	for _, succ := range b.Successors {
		i := slices.Index(succ.Predecessors, b)
		phis := slices.DeleteFunc(slices.Clone(succ.Values[:succ.phiIndex()]), func(phi *Value) bool {
			_, ok := s.memPhis[phi]
			return ok
		})

		// collect all phi args to be allocated at once
		args := make([]*Value, len(phis))
		for k, phi := range phis {
			args[k] = phi.Args[i]
		}
		s.reloadReads(b, len(s.vals), nil, args)

		for k, phi := range phis {
			phi.Args[i] = args[k]
		}
	}
}

func spillable(v *Value) bool {
	return v.NeedsRegister() && v.Loc.Kind == LocNone
}

func (s *spiller) resident(v *Value) bool {
	for _, h := range s.holders {
		if h == v {
			return true
		}
	}
	return false
}

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

		cheaper := cand.Rematerializable() && !victim.Rematerializable()
		same := cand.Rematerializable() == victim.Rematerializable()
		if cheaper || (same && d > far) {
			far, victim = d, cand
		}
	}
	return victim
}

func (s *spiller) take(v *Value, m register.Mask) {
	if r, ok := preferCallerSaved(m.Without(s.used())).One(); ok {
		s.holders[r] = v
	}
}

func preferCallerSaved(m register.Mask) register.Mask {
	if callerSaved := m.Intersect(register.CallerSaved); callerSaved.Count() > 0 {
		return callerSaved
	}
	return m
}

func (s *spiller) releaseEnded(b *Block, p int) {
	for r, h := range s.holders {
		if s.ranges[b][h].End <= p {
			delete(s.holders, r)
		}
	}
}

func (s *spiller) release(v *Value) {
	for r, h := range s.holders {
		if h == v {
			delete(s.holders, r)
			return
		}
	}
}

// evict frees victim's register, storing it to a slot first unless it can be rematerialized.
func (s *spiller) evict(victim *Value) {
	if victim.Rematerializable() {
		s.dropped = append(s.dropped, victim)
	} else if s.slots[victim] == nil {
		slot := s.f.newSlot(nil, victim.Type)
		var store *Value
		if victim.Op == OpPhi {
			// phis have to stay together at the top of the block, so a phi's store waits until after the last one
			store = s.f.insertValueAt(victim.Block.phiIndex(), OpStaticStore, victim.Type, victim.Block)
		} else {
			store = s.f.insertValueAfter(victim, OpStaticStore, victim.Type, victim.Block)
		}
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
	// b's phis read their operands in the predecessors, so reads start past them
	n := b.phiIndex()
	for q, u := range reads(s.vals[n:], b.Control) {
		if n+q > p && u == v {
			return n + q - p, true
		}
	}

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

				// s's phis read the operand for the edge from b as b ends
				i := slices.Index(s.Predecessors, b)
				for _, phi := range s.Values[:s.phiIndex()] {
					out[phi.Args[i]] = 0
				}
			}

			// anything still needed after b has to, at most, survive all of b
			in := make(map[*Value]int)
			for v, d := range out {
				in[v] = len(b.Values) + d
			}
			// b's phis read their args in the predecessor blocks, so read past them
			n := b.phiIndex()
			for p, v := range reads(b.Values[n:], b.Control) {
				if d, ok := in[v]; !ok || n+p < d {
					in[v] = n + p
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
