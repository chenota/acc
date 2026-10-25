package ssa

import (
	"errors"

	"github.com/chenota/acc/internal/register"
)

// regalloc colors SSA values with physical registers
func regalloc(f *Func) error {
	// spilling inserted stores and reloads, so the ranges are worked out fresh
	ranges := computeLiveRanges(f)

	// in definition order, everything live alongside a value and defined before it already has its register
	for v := range f.OrderedValues() {
		// skip values already put somewhere
		if v.Loc.Kind != LocNone || !v.NeedsRegister() {
			continue
		}

		reg, ok := pick(v, register.Allocatable.Without(ranges.held(v)))
		if !ok {
			return errors.New("regalloc: out of registers something has gone very wrong :D")
		}
		v.Loc = NewReg(reg)
	}

	return nil
}

// pick chooses v's register out of free with priority logic
func pick(v *Value, free register.Mask) (register.Register, bool) {
	// first try to pick the hinted-at register
	for h := range v.Hints() {
		if free.Contains(h) {
			return h, true
		}
	}

	// prefer caller-saved registers
	if callerSaved := free.Intersect(register.CallerSaved); callerSaved.Count() > 0 {
		return callerSaved.One()
	}

	// last resort pick an arbitrary available register
	return free.One()
}

type span struct {
	Start int
	End   int
}

func (s span) overlaps(o span) bool {
	return s.Start < o.End && o.Start < s.End
}

// liveRanges records, block by block, the span each value has to hold onto its register for
type liveRanges map[*Block]map[*Value]span

// computeLiveRanges finds each value's span in every block it's live in.
func computeLiveRanges(f *Func) liveRanges {
	lv := computeLiveness(f)
	ranges := make(liveRanges)

	for b := range f.OrderedBlocks() {
		spans := make(map[*Value]span)

		// touch records that v is live at position p
		touch := func(v *Value, p int) {
			if s, ok := spans[v]; ok {
				spans[v] = span{Start: min(s.Start, p), End: max(s.End, p)}
				return
			}
			spans[v] = span{Start: p, End: p}
		}

		// anything live coming in already holds its register before the first instruction
		for v := range lv[b].in {
			touch(v, -1)
		}

		// every value is live at its definition and at each point it is used
		for i, v := range b.Values {
			touch(v, i)
			for _, arg := range v.Args {
				touch(arg, i)
			}

			// parameters and the closure environment occupy their registers before the function's first instruction
			if v.Op == OpParam || v.Op == OpClosurePtr {
				touch(v, -1)
			}

			// a call result is live from the call that left it in its register
			if v.Op == OpCallResult {
				touch(v, spans[v.Args[0]].Start)
			}

			// ensure the right operand of a bop is live alongside the result so they don't get mapped to the same place
			if v.IsBinaryOp() {
				touch(v.Args[1], i+1)
			}
		}

		// the terminator reads control values right after the last instruction
		for _, c := range b.Control {
			touch(c, len(b.Values))
		}

		// anything still needed once b is finished
		for v := range lv[b].out {
			touch(v, len(b.Values)+1)
		}

		ranges[b] = spans
	}
	return ranges
}

// held is every register held by another value while v is live
func (r liveRanges) held(v *Value) register.Mask {
	var m register.Mask
	for _, spans := range r {
		if s, ok := spans[v]; ok {
			m = m.Union(heldWithin(spans, v, s))
		}
	}
	return m
}

// heldAt is every register something holds across position p of b
func (r liveRanges) heldAt(b *Block, p int) register.Mask {
	return heldWithin(r[b], nil, span{Start: p, End: p + 1})
}

// heldWithin is every register something other than v holds during s
func heldWithin(spans map[*Value]span, v *Value, s span) register.Mask {
	var m register.Mask
	for u, su := range spans {
		if u == v {
			continue
		}

		// a value occupies its register for its whole span
		if u.Loc.Kind == LocRegister && s.overlaps(su) {
			m = m.Include(u.Loc.Reg)
		}

		// a call only clobbers registers at the instant it runs
		if u.IsCall() && s.overlaps(span{Start: su.Start, End: su.Start + 1}) {
			m = m.Union(u.Clobbers())
		}
	}
	return m
}
