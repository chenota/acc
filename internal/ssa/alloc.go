package ssa

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/chenota/acc/internal/target"
)

// allocate gives every value a register, spilling with the MIN algorithm wherever there aren't enough to go around.
func allocate(f *Func) error {
	s := newSpiller(f)

	for b := range f.OrderedBlocks() {
		s.holders = s.enter(b)

		// evict and reload insert into b.Values, so walk and scan a copy taken before they do
		s.vals = slices.Clone(b.Values)

		phis := s.vals[:b.phiIndex()]
		if err := s.placePhis(b, phis); err != nil {
			return err
		}
		s.entries[b] = maps.Clone(s.holders)

		for _, p := range b.Predecessors {
			if err := s.finishEdge(p, b); err != nil {
				return err
			}
		}

		for p := len(phis); p < len(s.vals); p++ {
			v := s.vals[p]

			if err := s.reloadOperands(b, p, v); err != nil {
				return err
			}

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

		if err := s.reloadControl(b); err != nil {
			return err
		}

		s.exits[b] = blockExit{holders: s.holders, vals: s.vals}

		// a loop header is walked before the blocks that jump back to it, so those edges finish here
		for _, h := range b.Successors {
			if err := s.finishEdge(b, h); err != nil {
				return err
			}
		}
	}

	// once every read of a phi waiting in a slot is a reload, nothing reads the phi itself
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

	uses  nextUses // what's live at each block boundary, and how far away its next read is
	avail map[*Value]target.Mask

	// where each value holds its register, kept current as reloads are added
	ranges liveRanges

	holders map[target.Register]*Value
	entries map[*Block]map[target.Register]*Value // what each block expects in its registers as it starts
	exits   map[*Block]blockExit
	regs    map[*Value]target.Register // the register each value lives in whenever it isn't spilled
	slots   map[*Value]*Slot
	dropped []*Value

	// the block being walked, as it was before spilling inserted anything
	vals []*Value
}

type blockExit struct {
	holders map[target.Register]*Value
	vals    []*Value // the block as the walk saw it, before spilling inserted anything
}

func newSpiller(f *Func) *spiller {
	uses := computeNextUses(f)
	s := &spiller{
		f:       f,
		uses:    uses,
		ranges:  computeLiveRanges(f, uses),
		avail:   make(map[*Value]target.Mask),
		entries: make(map[*Block]map[target.Register]*Value),
		exits:   make(map[*Block]blockExit),
		regs:    make(map[*Value]target.Register),
		slots:   make(map[*Value]*Slot),
	}

	// TODO: recompute these as spilling shortens ranges, since the ranges from before the walk rule out more than they need to
	for v := range f.UnorderedValues() {
		if spillable(v) {
			s.avail[v] = v.allowed().Without(s.ranges.blockedFor(v))
		}
	}

	return s
}

// enter returns the values that b's predecessors left in registers.
// Based on Braun & Hack, "Register Spilling and Live-Range Splitting for SSA-Form Programs".
func (s *spiller) enter(b *Block) map[target.Register]*Value {
	// TODO: pick a loop header's entry set from what the loop reads rather than inheriting it
	var regs map[target.Register]*Value
	for _, p := range b.Predecessors {
		exit, ok := s.exits[p]
		if !ok {
			continue
		}

		if regs == nil {
			regs = maps.Clone(exit.holders)
			maps.DeleteFunc(regs, func(_ target.Register, v *Value) bool { return !s.ranges.liveOnEntry(b, v) })
			continue
		}

		// a value only ever lives in one register, so it's resident in b if every predecessor left it there
		maps.DeleteFunc(regs, func(r target.Register, v *Value) bool { return exit.holders[r] != v })
	}

	if regs == nil {
		regs = make(map[target.Register]*Value)
	}
	return regs
}

// reloadOperands brings v's spilled operands back into registers just before v reads them at position p of b
func (s *spiller) reloadOperands(b *Block, p int, v *Value) error {
	for i, a := range v.Args {
		if !spillable(a) || s.resident(a) {
			continue
		}

		reload := s.reload(b, b.indexOf(v), a)
		v.setArg(i, reload)
		if err := s.assignAt(b, p, reload, v.Args); err != nil {
			return err
		}

		// the reload holds its register only for the read it supports
		s.ranges[b][reload] = span{Start: p, End: readEnd(v, i, p)}
	}
	return nil
}

// reloadControl brings b's spilled control values back into registers for its terminator, which reads them as b ends
func (s *spiller) reloadControl(b *Block) error {
	control := slices.Clone(b.Control)
	for i, c := range control {
		if !spillable(c) || s.resident(c) {
			continue
		}

		control[i] = s.reload(b, len(b.Values), c)
		b.setControl(control...)
		if err := s.assignAt(b, len(s.vals), control[i], control); err != nil {
			return err
		}
	}
	return nil
}

// assignAt gives v, already wired into whatever reads it, a register free at position p of b that its readers can read
// it from
func (s *spiller) assignAt(b *Block, p int, v *Value, keep []*Value) error {
	m := s.scratch(b, p).Intersect(v.allowed())
	s.makeRoom(b, p, m, keep)
	return s.assign(v, m)
}

// placePhis gives b's phis their registers as b starts, or a slot to wait in for those that don't fit
func (s *spiller) placePhis(b *Block, phis []*Value) error {
	order := slices.Clone(phis)
	slices.SortStableFunc(order, func(u, w *Value) int { return s.avail[u].Count() - s.avail[w].Count() })

	for _, phi := range order {
		if !spillable(phi) {
			continue
		}

		// give up whatever is read furthest away, which is phi itself if nothing resident is read later
		avail := s.avail[phi]
		for avail.Without(s.used()).Count() == 0 {
			victim := s.pickVictim(b, -1, avail, nil)
			if victim == nil || (!victim.Rematerializable() && s.distance(b, victim) <= s.distance(b, phi)) {
				break
			}

			// a phi given up before anything reads it can wait in a slot from the start
			if victim.Op == OpPhi && victim.Block == b {
				s.keepInSlot(victim)
			} else {
				s.evict(b, -1, victim)
			}
		}

		if avail.Without(s.used()).Count() == 0 {
			s.keepInSlot(phi)
			continue
		}
		if err := s.assign(phi, avail); err != nil {
			return err
		}
	}

	return nil
}

// keepInSlot has phi wait in a slot instead of a register; its predecessors store their operands there.
func (s *spiller) keepInSlot(phi *Value) {
	s.release(phi)
	delete(s.regs, phi)

	slot := s.f.newSlot(nil, phi.Type)
	s.slots[phi] = slot
	phi.Value = slot
	s.dropped = append(s.dropped, phi)
}

// distance is how far into b v is next read, from before b's first instruction
func (s *spiller) distance(b *Block, v *Value) int {
	if d, ok := s.nextUse(b, s.vals, -1, v); ok {
		return d
	}
	return math.MaxInt
}

// finishEdge leaves everything where b expects it on the way from p, once both ends of the edge have been walked
func (s *spiller) finishEdge(p, b *Block) error {
	exit, walked := s.exits[p]
	entry, entered := s.entries[b]
	if !walked || !entered {
		return nil
	}

	holders, vals := s.holders, s.vals
	defer func() { s.holders, s.vals = holders, vals }()
	s.holders, s.vals = maps.Clone(exit.holders), exit.vals
	end := len(s.vals)

	i := slices.Index(b.Predecessors, p)
	phis := b.Values[:b.phiIndex()]

	// across a back edge a phi can read another phi of b, which has to see that phi's slot before this edge writes it
	for _, phi := range phis {
		if a := phi.Args[i]; a != phi && a.Block == b && waitsInSlot(a) {
			if err := s.saveOperand(p, end, phi, i); err != nil {
				return err
			}
		}
	}

	// a phi in a slot has its operand stored there, which only needs a register for the store to read it
	for _, phi := range phis {
		arg := phi.Args[i]
		if !waitsInSlot(phi) || arg == phi {
			continue
		}

		store := s.f.appendValue(OpStaticStore, phi.Type, p)
		store.setArgs(arg)
		store.Value = phi.Slot()
		if err := s.reloadOperands(p, end, store); err != nil {
			return err
		}
		phi.setArg(i, store.Args[0])
		if store.Args[0] != arg {
			s.release(store.Args[0])
		}
	}

	// the stores can give up anything to read their operands, so only now is it clear what's left to put back
	lost := func(phi *Value) bool {
		a := phi.Args[i]
		return !waitsInSlot(phi) && spillable(a) && !s.resident(a)
	}
	missing := slices.DeleteFunc(slices.Sorted(maps.Keys(entry)), func(r target.Register) bool {
		return s.resident(entry[r]) || slices.Contains(phis, entry[r])
	})
	if len(missing) == 0 && !slices.ContainsFunc(phis, lost) {
		return nil
	}

	// a restore can overwrite what a phi move reads, so the moves happen on the way into a block of the edge's own
	edge := s.f.splitEdge(p, b)
	for _, phi := range phis {
		if lost(phi) {
			phi.setArg(i, s.restore(edge, phi.Args[i], s.regs[phi]))
		} else if !waitsInSlot(phi) {
			s.stagePhi(edge, phi, i)
		}
	}
	for _, r := range missing {
		s.restore(edge, entry[r], r)
	}

	return nil
}

// stagePhi has edge take the ith operand of phi into phi's register, leaving nothing to move on the way out of edge
func (s *spiller) stagePhi(edge *Block, phi *Value, i int) {
	stage := s.f.insertValueFront(OpPhi, phi.Type, edge)
	stage.setArgs(phi.Args[i])
	phi.setArg(i, stage)

	if r, ok := s.regs[phi]; ok {
		s.regs[stage] = r
	}
}

// saveOperand moves the ith operand of phi to a slot of its own, leaving what p reads it from free to be written
func (s *spiller) saveOperand(p *Block, end int, phi *Value, i int) error {
	a := phi.Args[i]

	load := s.reload(p, len(p.Values), a)
	if err := s.assignAt(p, end, load, nil); err != nil {
		return err
	}

	s.slots[load] = s.f.newSlot(nil, a.Type)
	store := s.f.appendValue(OpStaticStore, a.Type, p)
	store.setArgs(load)
	store.Value = s.slots[load]

	s.release(load)
	phi.setArg(i, load)
	return nil
}

// restore has edge put a back in r
func (s *spiller) restore(edge *Block, a *Value, r target.Register) *Value {
	v := s.reload(edge, len(edge.Values), a)
	s.regs[v] = r
	return v
}

func spillable(v *Value) bool {
	return v.NeedsRegister() && v.Loc.Kind == LocNone
}

func waitsInSlot(v *Value) bool {
	return v.Op == OpPhi && v.Slot() != nil
}

func (s *spiller) resident(v *Value) bool {
	r, ok := s.regs[v]
	return ok && s.holders[r] == v
}

func (s *spiller) used() target.Mask {
	var m target.Mask
	for r := range s.holders {
		m = m.Include(r)
	}
	return m
}

// scratch is the set of registers nothing has been pinned to across position p of b.
func (s *spiller) scratch(b *Block, p int) target.Mask {
	return s.f.Target.Allocatable.Without(s.ranges.blockedAt(b, p))
}

func (s *spiller) makeRoom(b *Block, p int, m target.Mask, operands []*Value) {
	for m.Without(s.used()).Count() == 0 {
		victim := s.pickVictim(b, p, m, operands)
		if victim == nil {
			return
		}
		s.evict(b, p, victim)
	}
}

func (s *spiller) pickVictim(b *Block, p int, m target.Mask, operands []*Value) *Value {
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

// assign gives v a free register out of m for good, preferring one that saves a move
func (s *spiller) assign(v *Value, m target.Mask) error {
	free := m.Without(s.used())

	r, ok := preferCallerSaved(free, s.f.Target).One()
	for _, p := range s.preferences(v) {
		if free.Contains(p) {
			r, ok = p, true
			break
		}
	}
	if !ok {
		return fmt.Errorf("allocate: no register for v%d in block %d of %s", v.Id, v.Block.Id, s.f.Name())
	}

	s.holders[r] = v
	s.regs[v] = r
	return nil
}

// preferences ranks the registers v would rather live in, most wanted first
func (s *spiller) preferences(v *Value) []target.Register {
	votes := make(map[target.Register]int)
	vote := func(u *Value) {
		if r, ok := s.regs[u]; ok {
			votes[r] += 1
		} else if u.Loc.Kind == LocRegister {
			votes[u.Loc.Reg] += 1
		}
	}

	for _, u := range v.users {
		if u.Op == OpCopy || u.Op == OpPhi {
			vote(u)
		}
	}
	if v.Op == OpCopy || v.Op == OpPhi {
		for _, a := range v.Args {
			vote(a)
		}
	}

	prefs := slices.Collect(maps.Keys(votes))
	slices.SortFunc(prefs, func(a, b target.Register) int { return cmp.Or(votes[b]-votes[a], int(a)-int(b)) })
	return prefs
}

func preferCallerSaved(m target.Mask, t *target.Target) target.Mask {
	if callerSaved := m.Intersect(t.CallerSaved); callerSaved.Count() > 0 {
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
	if s.resident(v) {
		delete(s.holders, s.regs[v])
	}
}

// evict frees victim's register at position p of b, storing it to a slot first unless it can be rematerialized or
// nothing reads it again.
func (s *spiller) evict(b *Block, p int, victim *Value) {
	_, read := s.nextUse(b, s.vals, p, victim)
	if victim.Rematerializable() {
		s.dropped = append(s.dropped, victim)
	} else if read && s.slots[victim] == nil {
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

// reload brings evicted a back at index i of b
func (s *spiller) reload(b *Block, i int, a *Value) *Value {
	if a.Rematerializable() {
		r := s.f.insertValueAt(i, a.Op, a.Type, b)
		r.Value = a.Value
		r.Offset = a.Offset
		return r
	}

	r := s.f.insertValueAt(i, OpStaticLoad, a.Type, b)
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

	if d, ok := s.uses[b].out[v]; ok {
		return (len(vals) - p) + d, true
	}

	return 0, false
}
