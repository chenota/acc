package ssa

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/chenota/acc/internal/iterutil"
	"github.com/chenota/acc/internal/register"
)

// allocate gives every value a register, spilling with the MIN algorithm wherever there aren't enough to go around.
func allocate(f *Func) error {
	s := newSpiller(f)

	for b := range f.OrderedBlocks() {
		s.holders = s.enter(b)

		// evict and reload insert into b.Values, so walk and scan a copy taken before they do
		s.vals = slices.Clone(b.Values)

		// phis take their registers on the way into b, and their operands are read at the end of each predecessor
		phis := s.vals[:b.phiIndex()]
		if err := s.placePhis(b, phis); err != nil {
			return err
		}

		for p := len(phis); p < len(s.vals); p++ {
			v := s.vals[p]

			reads, err := s.reloadReads(b, p, v, v.Args)
			if err != nil {
				return err
			}
			v.setArgs(reads...)

			s.releaseEnded(b, p)

			if !spillable(v) {
				continue
			}

			avail := s.avail[v]
			s.makeRoom(b, p, avail, v.Args)
			if err := s.assign(v, avail); err != nil {
				return err
			}
		}

		if err := s.reloadPhiArgs(b); err != nil {
			return err
		}

		control, err := s.reloadReads(b, len(s.vals), nil, b.Control)
		if err != nil {
			return err
		}
		b.setControl(control...)

		// TODO: when b ends a loop body, restore each value the loop header holds on entry but b doesn't, from its slot
		// into its own register, as part of the edge's parallel copy
		s.exits[b] = s.holders
	}

	// drop operands on all memory phis to prevent references from other memory phis from making them appear alive
	for _, v := range s.dropped {
		if s.memPhis[v] {
			v.setArgs()
		}
	}

	for _, v := range s.dropped {
		f.removeIfDead(v)
	}

	// spillable and the pinned registers go by Loc, so the walk's choices only go into it once the walk is done
	for v := range f.UnorderedValues() {
		if r, ok := s.regs[v]; ok {
			v.Loc = NewReg(r)
		}
	}

	return nil
}

type spiller struct {
	f *Func

	// fixed before the walk starts
	uses  nextUses // how far past each block's end every value it leaves live is next read
	avail map[*Value]register.Mask

	// where each value holds its register, kept current as reloads are added
	ranges liveRanges

	// what the walk has decided so far
	holders map[register.Register]*Value
	exits   map[*Block]map[register.Register]*Value
	regs    map[*Value]register.Register // the register each value lives in whenever it isn't spilled
	slots   map[*Value]*Slot
	dropped []*Value
	memPhis map[*Value]bool

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
		regs:    make(map[*Value]register.Register),
		slots:   make(map[*Value]*Slot),
		memPhis: make(map[*Value]bool),
	}

	// TODO: recompute these as spilling shortens ranges, since the ranges from before the walk rule out more than they need to
	for v := range f.UnorderedValues() {
		if spillable(v) {
			s.avail[v] = register.Allocatable.Without(s.ranges.blockedFor(v))
		}
	}

	// need to pick which phis go in memory beforehand so predecessors know where to put phi args
	s.pickMemoryPhis()

	if s.storeMemoryPhiArgs() {
		// the new stores shift positions, leaving ranges and uses stale
		s.ranges = computeLiveRanges(f)
		s.uses = computeNextUses(f)
	}

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
		dist := make(map[*Value]int)
		for v := range s.ranges[b] {
			if !s.ranges.liveOnEntry(b, v) || !spillable(v) {
				continue
			}
			live = append(live, v)
			dist[v] = math.MaxInt
			if d, ok := s.nextUse(b, b.Values, -1, v); ok {
				dist[v] = d
			}
		}
		slices.SortFunc(live, func(u, v *Value) int {
			return cmp.Or(cmp.Compare(dist[u], dist[v]), cmp.Compare(u.Id, v.Id))
		})

		// hand out registers the way the walk will
		s.holders = make(map[register.Register]*Value)
		free := s.scratch(b, -1)
		for _, v := range live {
			if _, ok := s.take(v, s.avail[v].Intersect(free)); ok || v.Op != OpPhi || v.Block != b {
				continue
			}

			s.memPhis[v] = true
			s.slots[v] = s.f.newSlot(nil, v.Type)
			s.dropped = append(s.dropped, v)
		}
	}
}

// enter is values that b's predecessors left in registers
// TODO: a loop header is walked before its back edges, so choose its entry set up front instead
// Braun & Hack, "Register Spilling and Live-Range Splitting for SSA-Form Programs"
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

// reloadReads brings every spilled value in reads back into a register
func (s *spiller) reloadReads(b *Block, p int, at *Value, reads []*Value) ([]*Value, error) {
	reads = slices.Clone(reads)
	for i, a := range reads {
		if !spillable(a) || s.resident(a) {
			continue
		}

		m := s.scratch(b, p)
		s.makeRoom(b, p, m, reads)

		reload := s.reload(b, at, a)
		reads[i] = reload
		if err := s.assign(reload, m); err != nil {
			return nil, err
		}

		// the reload holds its register only for the read it supports
		end := p
		if at != nil {
			end = readEnd(at, i, p)
		}
		s.ranges[b][reload] = span{Start: p, End: end}
	}
	return reads, nil
}

// placePhis gives b's phis their registers as b starts
func (s *spiller) placePhis(b *Block, phis []*Value) error {
	order := slices.Clone(phis)
	slices.SortStableFunc(order, func(u, w *Value) int { return s.avail[u].Count() - s.avail[w].Count() })

	for _, phi := range order {
		if s.memPhis[phi] || !spillable(phi) {
			continue
		}

		// sharing a register with an operand makes the move on that operand's edge redundant
		// TODO: a back-edge operand is walked after its phi, so it should lean toward the phi's register instead
		for _, a := range phi.Args {
			if r, ok := s.regs[a]; ok {
				phi.RecordHint(r)
			} else if a.Loc.Kind == LocRegister {
				phi.RecordHint(a.Loc.Reg)
			}
		}

		// every phi takes its register on the way in, so giving one up wouldn't make room for another
		avail := s.avail[phi]
		s.makeRoom(b, slices.Index(phis, phi), avail, phis)
		if err := s.assign(phi, avail); err != nil {
			return err
		}
	}

	return nil
}

// storeMemoryPhiArgs writes a block's outgoing phi args to whatever slot it goes to
// TODO: across a back edge a phi can take another phi of the same block as an operand, so these stores have to join the
// edge's parallel copy, reading every operand before writing any slot
func (s *spiller) storeMemoryPhiArgs() bool {
	var stored bool
	for _, b := range s.f.Blocks {
		for _, succ := range b.Successors {
			for phi, arg := range succ.PhiArgsFrom(b) {
				if !s.memPhis[phi] {
					continue
				}

				store := s.f.appendValue(OpStaticStore, phi.Type, b)
				store.setArgs(arg)
				store.Value = s.slots[phi]
				stored = true
			}
		}
	}
	return stored
}

// reloadPhiArgs brings back every spilled operand b hands to its successors' register phis, which read them as b ends
func (s *spiller) reloadPhiArgs(b *Block) error {
	for _, succ := range b.Successors {
		var phis, args []*Value
		for phi, arg := range succ.PhiArgsFrom(b) {
			if !s.memPhis[phi] {
				phis = append(phis, phi)
				args = append(args, arg)
			}
		}
		args, err := s.reloadReads(b, len(s.vals), nil, args)
		if err != nil {
			return err
		}

		i := slices.Index(succ.Predecessors, b)
		for k, phi := range phis {
			phi.setArg(i, args[k])
		}
	}

	return nil
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
			return
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

		d, ok := s.nextUse(b, s.vals, p, cand)
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

// take gives v a free register out of m, preferring one v was hinted toward, and reports which one
func (s *spiller) take(v *Value, m register.Mask) (register.Register, bool) {
	free := m.Without(s.used())

	r, ok := preferCallerSaved(free).One()
	for h := range v.Hints() {
		if free.Contains(h) {
			r, ok = h, true
			break
		}
	}

	if ok {
		s.holders[r] = v
	}
	return r, ok
}

// assign takes v's register out of m for good
func (s *spiller) assign(v *Value, m register.Mask) error {
	r, ok := s.take(v, m)
	if !ok {
		return fmt.Errorf("allocate: no register for v%d in block %d of %s", v.Id, v.Block.Id, s.f.Name())
	}

	s.regs[v] = r
	return nil
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
		store.setArgs(victim)
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
func (s *spiller) nextUse(b *Block, vals []*Value, p int, v *Value) (int, bool) {
	for q, u := range reads(vals, b.Control) {
		if q > p && u == v {
			return q - p, true
		}
	}

	if d, ok := s.uses[b][v]; ok {
		return (len(vals) - p) + d, true
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
				for _, a := range s.PhiArgsFrom(b) {
					out[a] = 0
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
