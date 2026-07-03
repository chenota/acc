package ssa

func negSquash(f *Func) {
	for _, value := range squashableValues(f) {
		// transform value's op in-place
		if value.Op == OpAdd {
			value.Op = OpSubtract
		} else {
			value.Op = OpAdd
		}

		negate := value.Args[1]
		// replace the negate operand with the value it negates
		value.Args[1] = negate.Args[0]

		f.removeValue(negate)
	}
}

func squashableValues(f *Func) []*Value {
	vals := make([]*Value, 0)

	for _, value := range f.OrderedValues() {
		// want to squash x + -y or x - -y
		if (value.Op == OpAdd || value.Op == OpSubtract) && value.Args[1].Op == OpNegate {
			vals = append(vals, value)
		}
	}

	return vals
}
