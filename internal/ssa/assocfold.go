package ssa

import "slices"

func associativeFold(f *Func) {
	for _, root := range associativeRoots(f) {
		chain := associativeChain(root)
		consts, vars := leaves(chain)
		if len(consts) < 2 {
			continue
		}

		foldedValue := foldConsts(consts, root.Op)
		for _, c := range consts {
			f.removeValue(c)
		}

		cores := cores(chain)
		if len(vars) == 0 {
			foldConstantChain(f, root, foldedValue, cores)
		} else {
			rewireMixedChain(f, root, foldedValue, cores, vars)
		}
	}
}

// foldConstantChain handles chains where every leaf is a constant.
// It substitutes root with a single folded constant and removes the remaining cores.
func foldConstantChain(f *Func, root *Value, foldedValue any, cores []*Value) {
	newConst := f.newValue(OpLiteral, root.Type, root.Block)
	newConst.Value = foldedValue
	f.substituteValue(root, newConst)
	for _, c := range cores[1:] {
		f.removeValue(c)
	}
}

// rewireMixedChain handles chains with at least one variable leaf.
// It inserts a folded constant and rewires the kept cores around the variable leaves.
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
		f.removeValue(c)
	}
}

func associativeRoots(f *Func) []*Value {
	roots := make([]*Value, 0)

	for _, v := range f.values() {
		if v.IsAssociativeOp() && !isSubOperation(v) {
			roots = append(roots, v)
		}
	}

	return roots
}

// isSubOperation checks if this operation lives under another operation of the same type
func isSubOperation(v *Value) bool {
	for _, val := range v.Block.Values {
		if val == v || val.Op != v.Op {
			continue
		}

		if slices.Contains(val.Args, v) {
			return true
		}
	}

	return false
}

func associativeChain(root *Value) []*Value {
	op := root.Op
	chain := make([]*Value, 0)

	for {
		chain = append(chain, root)

		if root.Op != op {
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

func foldConsts(vals []*Value, op Op) any {
	var acc any

	for _, v := range vals {
		if acc == nil {
			acc = v.Value
		} else {
			acc, _ = evaluateBop(op, v.Type, acc, v.Value)
		}
	}

	return acc
}
