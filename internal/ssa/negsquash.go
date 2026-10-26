package ssa

func negSquash(f *Func) error {
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

		// other users may still read the negate
		f.removeIfDead(negate)
	}

	return nil
}

func squashableValues(f *Func) []*Value {
	vals := make([]*Value, 0)

	for value := range f.OrderedValues() {
		// want to squash x + -y or x - -y
		if (value.Op == OpAdd || value.Op == OpSubtract) && value.Args[1].Op == OpNegate {
			vals = append(vals, value)
		}
	}

	return vals
}
