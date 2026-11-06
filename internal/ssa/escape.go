package ssa

// slotEscapes determines the frame slots whose storage outlives the call or the loop iteration that declared them.
func (f *Func) slotEscapes() []*Slot {
	sinks := f.heapSeeds()
	for v := range f.UnorderedValues() {
		if v.Op == OpResult {
			// slots can escape out of function results
			sinks = append(sinks, seed{valueNode(v.Args[0]), 0})
		}
	}

	result := walk(f, sinks)
	carried := f.carriedSlots()

	var escaped []*Slot
	for _, s := range f.Slots {
		if d, ok := result[slotNode(s)]; carried[s] || (ok && d < 0) {
			escaped = append(escaped, s)
		}
	}

	return escaped
}

// carriedSlots finds the slots declared in a loop whose address outlives a single loop iteration
func (f *Func) carriedSlots() map[*Slot]bool {
	nearestDominators := f.nearestDominatingBlocks()
	carried := make(map[*Slot]bool)

	for b, l := range f.Loops() {
		// only want to analyze loop headers
		if b != l.Header {
			continue
		}

		var seeds []seed
		for _, phi := range b.Values[:b.phiIndex()] {
			// header phi nodes are potential escape points

			/*
				NOTE TO SELF:

				The point of this check is to capture variable usage that could result in a loop slot
				outliving a specific loop iteration. For example,

				let x *int = nil;
				loop {
					let j = whatever;
						if something {
						x = &j;
					}
					continue loop logic...
				}
				use x down here...

				We'd want j's slot marked as carried from the loop, since it escapes through x. It's clear to see in this
				example that x, as seen from the perspective of the loop header, is a phi value since it can come from
				either the declaration outside of the loop, or from the update within the loop. So in short what a phi node on the
				loop header is telling us is that there's a variable that is declared outside of the loop and updated inside the loop
				which is a little sus so it needs to be checked as a potential escape for a loop slot.
			*/

			seeds = append(seeds, seed{valueNode(phi), 0})
		}
		for s, blk := range nearestDominators {
			// slots declared outside of the loop are potential escape points
			if !l.contains(blk) {
				seeds = append(seeds, seed{slotNode(s), 0})
			}
		}

		result := walk(f, seeds)
		for s, blk := range nearestDominators {
			// if a slot escapes and is declared in the loop, then it is carried
			if d, ok := result[slotNode(s)]; ok && d < 0 && l.contains(blk) {
				carried[s] = true
			}
		}
	}

	return carried
}

func (f *Func) escapeSummary() []paramSummary {
	if f.escapes != nil {
		// already calculated, return memoized summary
		return f.escapes
	}

	if f.escapeInProgress {
		// rescursive shenanigans going on, assume that everything leaks
		// TODO: solve the cycle instead so recursive functions don't leak every parameter
		return f.conservativeSummary()
	}

	f.escapeInProgress = true
	defer func() { f.escapeInProgress = false }()

	// figure out heap escapes
	heapSinks := f.heapSeeds()
	for _, s := range f.slotEscapes() {
		// include all escaped slots of this function in heap analysis
		heapSinks = append(heapSinks, seed{slotNode(s), -1})
	}
	heapResults := walk(f, heapSinks)

	// figure out result escapes
	resultResults := make([]map[node]int, len(f.Results))
	for i := range f.Results {
		var resultSinks []seed
		for _, r := range f.resultValues(i) {
			resultSinks = append(resultSinks, seed{valueNode(r.Args[0]), 0})
		}
		resultResults[i] = walk(f, resultSinks)
	}

	// empty escapes map
	f.escapes = make([]paramSummary, len(f.Params))

	// fill out each parameter's entry
	paramValues := f.paramValues()
	for i := range len(f.Params) {
		if d, ok := heapResults[valueNode(paramValues[i])]; ok {
			f.escapes[i].heap = new(d)
		}
		for j := range len(f.Results) {
			if d, ok := resultResults[j][valueNode(paramValues[i])]; ok {
				f.escapes[i].results = append(f.escapes[i].results, new(d))
			} else {
				f.escapes[i].results = append(f.escapes[i].results, nil)
			}
		}
	}

	return f.escapes
}

// conservativeSummary assumes every parameter of f reaches the heap itself.
func (f *Func) conservativeSummary() []paramSummary {
	summary := make([]paramSummary, len(f.Params))
	for i := range summary {
		summary[i] = paramSummary{heap: new(0), results: make([]*int, len(f.Results))}
	}
	return summary
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
	heap    *int   // lowest deref count from the parameter to the heap (nil if this param never reaches the heap)
	results []*int // lowest deref count from the parameter to each result leaf (nil if no connection between this param and result)
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
		case OpPhi:
			for _, a := range v.Args {
				relax(valueNode(a), d)
			}
		case OpCallResult:
			call := v.Args[0]
			if call.Op != OpStaticCall {
				break // closure call's arguments assumed to reach the heap
			}
			k := v.Value.(int) // index of this result
			for i, s := range call.Callee().escapeSummary() {
				if lvl := s.results[k]; lvl != nil {
					relax(valueNode(call.Args[i]), *lvl+d)
				}
			}
		}
	}

	return derefs
}
