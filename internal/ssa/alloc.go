package ssa

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/chenota/acc/internal/register"
)

// allocate gives every value a register, spilling with the MIN algorithm wherever there aren't enough to go around.
func allocate(f *Func) error {
	s := newSpiller(f)

	for b := range f.OrderedBlocks() {
		s.holders = s.enter(b)

		// evict and reload insert into b.Values, so walk and scan a copy taken before they do
		s.vals = slices.Clone(b.Values)

		// phis take their places on the way into b, and then each predecessor hands them their operands as it ends
		phis := s.vals[:b.phiIndex()]
		if err := s.placePhis(b, phis); err != nil {
			return err
		}
		for _, p := range b.Predecessors {
			if err := s.finishEdge(p, b, phis); err != nil {
				return err
			}
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

		control, err := s.reloadReads(b, len(s.vals), nil, b.Control)
		if err != nil {
			return err
		}
		b.setControl(control...)

		// TODO: when b ends a loop body, the loop header was walked first, so finish the edge back to it here instead,
		// restoring each value the header holds on entry but b doesn't from its slot into its own register
		s.exits[b] = blockExit{holders: s.holders, vals: s.vals}
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

	// fixed before the walk starts
	uses  nextUses // what's live at each block boundary, and how far away its next read is
	avail map[*Value]register.Mask

	// where each value holds its register, kept current as reloads are added
	ranges liveRanges

	// what the walk has decided so far
	holders map[register.Register]*Value
	exits   map[*Block]blockExit
	regs    map[*Value]register.Register // the register each value lives in whenever it isn't spilled
	slots   map[*Value]*Slot
	dropped []*Value

	// the block being walked, as it was before spilling inserted anything
	vals []*Value
}

// blockExit is where the walk left off at the end of a block
type blockExit struct {
	holders map[register.Register]*Value
	vals    []*Value // the block as the walk saw it, before spilling inserted anything
}

func newSpiller(f *Func) *spiller {
	uses := computeNextUses(f)
	s := &spiller{
		f:      f,
		uses:   uses,
		ranges: computeLiveRanges(f, uses),
		avail:  make(map[*Value]register.Mask),
		exits:  make(map[*Block]blockExit),
		regs:   make(map[*Value]register.Register),
		slots:  make(map[*Value]*Slot),
	}

	// TODO: recompute these as spilling shortens ranges, since the ranges from before the walk rule out more than they need to
	for v := range f.UnorderedValues() {
		if spillable(v) {
			s.avail[v] = register.Allocatable.Without(s.ranges.blockedFor(v))
		}
	}

	return s
}

// enter is values that b's predecessors left in registers
//
// TODO: a loop header is walked before its back edges, so choose its entry set up front instead
// Braun & Hack, "Register Spilling and Live-Range Splitting for SSA-Form Programs"
func (s *spiller) enter(b *Block) map[register.Register]*Value {
	var regs map[register.Register]*Value
	for _, p := range b.Predecessors {
		exit, ok := s.exits[p]
		if !ok {
			continue
		}

		if regs == nil {
			regs = maps.Clone(exit.holders)
			maps.DeleteFunc(regs, func(_ register.Register, v *Value) bool { return !s.ranges.liveOnEntry(b, v) })
			continue
		}

		// a value only ever lives in one register, so it's resident in b if every predecessor left it there
		maps.DeleteFunc(regs, func(r register.Register, v *Value) bool { return exit.holders[r] != v })
	}

	if regs == nil {
		regs = make(map[register.Register]*Value)
	}
	return regs
}

// reloadReads brings every spilled value in reads back into a register, without giving up any of reads or keep for room
func (s *spiller) reloadReads(b *Block, p int, at *Value, reads []*Value, keep ...*Value) ([]*Value, error) {
	reads = slices.Clone(reads)
	for i, a := range reads {
		if !spillable(a) || s.resident(a) {
			continue
		}

		m := s.scratch(b, p)
		s.makeRoom(b, p, m, slices.Concat(reads, keep))

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

// placePhis gives b's phis their registers as b starts, or a slot to wait in for those that don't fit
func (s *spiller) placePhis(b *Block, phis []*Value) error {
	order := slices.Clone(phis)
	slices.SortStableFunc(order, func(u, w *Value) int { return s.avail[u].Count() - s.avail[w].Count() })

	for _, phi := range order {
		if !spillable(phi) {
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
				s.evict(victim)
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

// keepInSlot has phi wait in a slot of its own instead of a register, which its predecessors store to as they end
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

// finishEdge hands b's phis their operands at the end of p
//
// TODO: across a back edge a phi can take another phi of the same block as an operand, so every operand has to be read
// before any slot is written. Also the loop header's entry set can hold every register, leaving no room to reload a
// slot's operand
func (s *spiller) finishEdge(p, b *Block, phis []*Value) error {
	exit, ok := s.exits[p]
	if !ok || len(phis) == 0 {
		return nil
	}

	entry, vals := s.holders, s.vals
	defer func() { s.holders, s.vals = entry, vals }()
	s.holders, s.vals = maps.Clone(exit.holders), exit.vals
	end := len(s.vals)

	// whatever b holds as it starts has to stay where b expects it
	var keep []*Value
	for _, v := range entry {
		if s.resident(v) {
			keep = append(keep, v)
		}
	}

	i := slices.Index(b.Predecessors, p)

	// a phi waiting in a slot has its operand stored there, which only needs a register while the store reads it
	for _, phi := range phis {
		if !waitsInSlot(phi) {
			continue
		}

		arg := phi.Args[i]
		in, err := s.reloadReads(p, end, nil, []*Value{arg}, keep...)
		if err != nil {
			return err
		}
		store := s.f.appendValue(OpStaticStore, phi.Type, p)
		store.setArgs(in[0])
		store.Value = phi.Slot()
		phi.setArg(i, in[0])
		if in[0] != arg {
			s.release(in[0])
		}
	}

	// the rest read their operands as the edge's parallel copy runs, so those all have to be in registers at once
	var inRegs, args []*Value
	for _, phi := range phis {
		if !waitsInSlot(phi) {
			inRegs = append(inRegs, phi)
			args = append(args, phi.Args[i])
		}
	}
	args, err := s.reloadReads(p, end, nil, args, keep...)
	if err != nil {
		return err
	}
	for k, phi := range inRegs {
		phi.setArg(i, args[k])
	}

	return nil
}

func spillable(v *Value) bool {
	return v.NeedsRegister() && v.Loc.Kind == LocNone
}

// waitsInSlot reports whether v is a phi that was given a slot to wait in instead of a register
func waitsInSlot(v *Value) bool {
	return v.Op == OpPhi && v.Slot() != nil
}

func (s *spiller) resident(v *Value) bool {
	r, ok := s.regs[v]
	return ok && s.holders[r] == v
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

// assign gives v a free register out of m for good, preferring one v was hinted toward
func (s *spiller) assign(v *Value, m register.Mask) error {
	free := m.Without(s.used())

	r, ok := preferCallerSaved(free).One()
	for h := range v.Hints() {
		if free.Contains(h) {
			r, ok = h, true
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
	if s.resident(v) {
		delete(s.holders, s.regs[v])
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

	if d, ok := s.uses[b].out[v]; ok {
		return (len(vals) - p) + d, true
	}

	return 0, false
}
