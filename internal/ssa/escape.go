package ssa

// slotEscapes determines the frame slots whose storage outlives the call.
func (f *Func) slotEscapes() []*Slot {
	var sinks []seed
	for v := range f.UnorderedValues() {
		switch {
		case v.Op == OpStore || v.Op == OpResult: // return values and stored-through-pointer values
			sinks = append(sinks, seed{valueNode(v.Args[0]), 0})
		case v.IsCall():
			// all call arguments are assumed to escape for the time being
			// TODO: use per-callee summaries for static calls
			for _, arg := range v.Args {
				sinks = append(sinks, seed{valueNode(arg), 0})
			}
		}
	}

	result := walk(f, sinks)

	var escaped []*Slot
	for _, s := range f.Slots {
		if d, ok := result[slotNode(s)]; ok && d < 0 {
			escaped = append(escaped, s)
		}
	}

	return escaped
}

// a node is either an ssa value or a frame slot
type node struct {
	value *Value
	slot  *Slot
}

func slotNode(s *Slot) node {
	return node{slot: s}
}

func valueNode(v *Value) node {
	return node{value: v}
}

// a seed is a node that reaches a sink directly, with a deref count tied to it so callers
// can use function summary results as a base for the seed
type seed struct {
	node   node
	derefs int
}

type paramSummary struct {
	heap    int   // lowest deref count from the parameter to the heap
	results []int // lowest deref count from the parameter to each result leaf
}

// walk returns the lowest deref count from each node (value/slot) to any of the seeded sinks
func walk(f *Func, seeds []seed) map[node]int {
	derefs := make(map[node]int)

	var queue []node
	relax := func(n node, d int) {
		// if we've already seen a worse case this doesn't need to be investigated
		if cur, seen := derefs[n]; seen && cur <= d {
			return
		}
		derefs[n] = d
		queue = append(queue, n)
	}

	// enqueue every sink to be looked at
	for _, s := range seeds {
		relax(s.node, s.derefs)
	}

	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		d := derefs[n]

		if n.slot != nil {
			// an escaping slot moves to the heap and becomes a sink itself, so what it holds starts over at zero.
			d = max(d, 0)

			// anything written to this slot at any point inherits its deficit
			for val := range f.SlotValues(n.slot) {
				if val.Op == OpStaticStore {
					relax(valueNode(val.Args[0]), d)
				}
			}
			continue
		}

		switch v := n.value; v.Op {
		case OpLocalAddr: // v = &slot, bump deficit of slot
			relax(slotNode(v.Slot()), d-1)
		case OpStaticLoad: // v = slot, slot inherits deficit of v
			relax(slotNode(v.Slot()), d)
		case OpLoad: // v = *args[0] alleviates deficit
			relax(valueNode(v.Args[0]), d+1)
		case OpCopy, OpFieldAddr: // v = args[0] (offset does not change the level), args[0] inherits deficit of v
			relax(valueNode(v.Args[0]), d)
		}
	}

	return derefs
}
