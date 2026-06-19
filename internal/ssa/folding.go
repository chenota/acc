package ssa

import "github.com/chenota/acc/internal/types"

func foldConstants(f *Func) {
	for _, b := range f.OrderedBlocks() {
		for _, v := range b.OrderedValues() {
			// exclude everything that isn't a bop with constant args; this will need expanded later
			if !v.IsBinaryOp() || !v.Args[0].IsConstant() || !v.Args[1].IsConstant() {
				continue
			}

			leftVal := v.Args[0].AuxInt
			rightVal := v.Args[1].AuxInt

			foldedVal, ok := evaluateBop(v.Op, v.Type, leftVal, rightVal)
			if !ok {
				continue
			}

			constOp := f.newValue(OpLiteral, v.Type, b)
			constOp.AuxInt = foldedVal

			f.substituteValue(v, constOp)
		}
	}
}

func evaluateBop(op Op, t *types.Type, left, right int64) (int64, bool) {
	if !t.IsConcreteNumeric() {
		return 0, false
	}

	switch op {
	case OpAdd:
		return left + right, true
	case OpSubtract:
		return left - right, true
	case OpMultiply:
		return left * right, true
	case OpDivide:
		if right != 0 {
			return left / right, true
		}
	}

	return 0, false
}
