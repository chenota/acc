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

	// enqueue both sink types (return values and stored-through-pointer values) to be looked at
	for block := range f.OrderedBlocks() {
		if block.Kind == BlockRet && block.Control != nil {
			relax(valueNode(block.Control), 0)
		}
	}
	for v := range f.UnorderedValues() {
		if v.Op == OpStore {
			relax(valueNode(v.Args[0]), 0)
		}
	}

	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		d := derefs[n]

		if n.slot != nil {
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
		case OpCopy: // v = args[0], args[0] in herits deficit of v
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
