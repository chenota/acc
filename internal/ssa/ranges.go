package ssa

import (
	"iter"

	"github.com/chenota/acc/internal/register"
)

type span struct {
	Start int
	End   int
}

func (s span) overlaps(o span) bool {
	return s.Start < o.End && o.Start < s.End
}

// liveRanges records, for each block, the span each value has to hold onto its register for.
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

		// every value is live at its definition and for as long as each of its reads needs it
		for i, v := range b.Values {
			touch(v, i)
			if v.Op != OpPhi {
				for j, arg := range v.Args {
					touch(arg, readEnd(v, j, i))
				}
			}

			// parameters, the closure environment, and phis occupy their registers before the block's first instruction
			if v.Op == OpParam || v.Op == OpClosurePtr || v.Op == OpPhi {
				touch(v, -1)
			}

			// a call result is live from the call that left it in its register
			if v.Op == OpCallResult {
				touch(v, spans[v.Args[0]].Start)
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

// readEnd is how long v's jth operand has to keep its register when v sits at position p
func readEnd(v *Value, j, p int) int {
	// the right operand of a bop is live alongside the result so they don't get mapped to the same place
	if v.IsBinaryOp() && j == 1 {
		return p + 1
	}
	return p
}

// liveOnEntry reports whether v already holds its register when b starts
func (r liveRanges) liveOnEntry(b *Block, v *Value) bool {
	s, ok := r[b][v]
	return ok && s.Start < 0
}

// blockedFor is every register v can't have, because something else occupies it somewhere v is live
func (r liveRanges) blockedFor(v *Value) register.Mask {
	var m register.Mask
	for _, spans := range r {
		if s, ok := spans[v]; ok {
			for u, regs := range occupants(spans, s) {
				// want to skip spans occupied by the value we're querying
				if u != v {
					m = m.Union(regs)
				}
			}
		}
	}
	return m
}

// blockedAt is every register something occupies across position p of b
func (r liveRanges) blockedAt(b *Block, p int) register.Mask {
	var m register.Mask
	for _, regs := range occupants(r[b], span{Start: p, End: p + 1}) {
		m = m.Union(regs)
	}
	return m
}

// occupants yields each value occupying registers at some point during s, along with the registers it occupies.
func occupants(spans map[*Value]span, s span) iter.Seq2[*Value, register.Mask] {
	return func(yield func(*Value, register.Mask) bool) {
		for u, su := range spans {
			var regs register.Mask

			// a value occupies its register for its whole span
			if u.Loc.Kind == LocRegister && s.overlaps(su) {
				regs = regs.Include(u.Loc.Reg)
			}

			// a call only clobbers registers at the instant it runs
			if u.IsCall() && s.overlaps(span{Start: su.Start, End: su.Start + 1}) {
				regs = regs.Union(u.Clobbers())
			}

			if regs.Count() > 0 && !yield(u, regs) {
				return
			}
		}
	}
}
