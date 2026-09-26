package ssa

// escapeAnalysis determines the frame slots whose storage outlives the call.
func escapeAnalysis(f *Func) []*Slot {
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

	// enqueue every sink type to be looked at
	for v := range f.UnorderedValues() {
		switch {
		case v.Op == OpResult || v.Op == OpStore: // return values and stored-through-pointer values
			relax(valueNode(v.Args[0]), 0)
		case v.IsCall():
			// all call arguments are assumed to escape for the time being
			// TODO: use per-callee summaries for static calls
			for _, arg := range v.Args {
				relax(valueNode(arg), 0)
			}
		}
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

	var escaped []*Slot
	for _, slot := range f.Slots {
		if d, ok := derefs[slotNode(slot)]; ok && d < 0 {
			escaped = append(escaped, slot)
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
