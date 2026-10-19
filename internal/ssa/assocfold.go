package ssa

import "slices"

func associativeFold(f *Func) {
	for _, root := range associativeRoots(f) {
		chain := associativeChain(f, root)
		consts, vars := leaves(chain)
		if len(consts) < 2 {
			continue
		}

		foldedValue, ok := foldConsts(consts, root.Op)
		if !ok {
			continue
		}

		cores := cores(chain)
		if len(vars) == 0 {
			foldConstantChain(f, root, foldedValue, cores)
		} else {
			rewireMixedChain(f, root, foldedValue, cores, vars)
		}

		for _, c := range consts {
			f.removeIfDead(c)
		}
	}
}

// foldConstantChain handles chains where every leaf is a constant.
func foldConstantChain(f *Func, root *Value, foldedValue any, cores []*Value) {
	newConst := f.newValue(OpLiteral, root.Type, root.Block)
	newConst.Value = foldedValue
	f.replaceValue(root, newConst)
	for _, c := range cores[1:] {
		f.removeIfDead(c)
	}
}

// rewireMixedChain handles chains with at least one variable leaf.
func rewireMixedChain(f *Func, root *Value, foldedValue any, cores []*Value, vars []*Value) {
	varCount := len(vars)

	newConst := f.insertValueBefore(root, OpLiteral, root.Type, root.Block)
	newConst.Value = foldedValue

	// the innermost kept core's left arg points to the first surplus core; redirect it to the first var
	cores[varCount-1].Args[0] = vars[0]

	// outermost core gets newConst on the right; each inner core gets the next var
	for i := range varCount {
		if i == 0 {
			cores[i].Args[1] = newConst
		} else {
			cores[i].Args[1] = vars[varCount-i]
		}
	}

	for _, c := range cores[varCount:] {
		f.removeIfDead(c)
	}
}

func associativeRoots(f *Func) []*Value {
	roots := make([]*Value, 0)

	for v := range f.OrderedValues() {
		if v.IsAssociativeOp() && !isSubOperation(f, v) {
			roots = append(roots, v)
		}
	}

	return roots
}

// isSubOperation checks if this operation's only use is another operation of the same type
func isSubOperation(f *Func, v *Value) bool {
	if f.useCount(v) != 1 {
		return false
	}

	for val := range f.UnorderedValues() {
		if val.Op == v.Op && slices.Contains(val.Args, v) {
			return true
		}
	}

	return false
}

func associativeChain(f *Func, root *Value) []*Value {
	op := root.Op
	chain := make([]*Value, 0)

	for {
		chain = append(chain, root)

		// a shared inner operation is a leaf
		if root.Op != op || (len(chain) > 1 && f.useCount(root) > 1) {
			break
		}

		root = root.Args[0]
	}

	return chain
}

func cores(chain []*Value) []*Value {
	core := make([]*Value, 0)

	for i, v := range chain {
		if i < len(chain)-1 {
			core = append(core, v)
		}
	}

	return core
}

func leaves(chain []*Value) ([]*Value, []*Value) {
	consts := make([]*Value, 0)
	vars := make([]*Value, 0)

	for i, v := range chain {
		var leaf *Value
		if i == len(chain)-1 {
			leaf = v
		} else {
			leaf = v.Args[1]
		}

		if leaf.IsConstant() {
			consts = append(consts, leaf)
		} else {
			vars = append(vars, leaf)
		}
	}

	return consts, vars
}

// foldConsts combines every constant leaf into one value
func foldConsts(vals []*Value, op Op) (any, bool) {
	var acc any

	for _, v := range vals {
		if acc == nil {
			acc = v.Value
			continue
		}

		folded, ok := evaluateBop(op, v.Type, acc, v.Value)
		if !ok {
			return nil, false
		}
		acc = folded
	}

	return acc, true
}
