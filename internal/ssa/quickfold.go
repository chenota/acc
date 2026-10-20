package ssa

import "github.com/chenota/acc/internal/types"

func quickFold(f *Func) {
	for v := range f.OrderedValues() {
		if !v.IsBinaryOp() && !v.IsComparison() {
			continue
		}

		left := v.Args[0]
		right := v.Args[1]

		if !(left.IsConstant() && right.IsConstant()) {
			continue
		}

		foldedVal, ok := evaluateBop(v.Op, left.Type, left.Value, right.Value)
		if !ok {
			continue
		}

		constOp := f.newValue(OpLiteral, v.Type, v.Block)
		constOp.Value = foldedVal

		// replace the top-level value with the new constant
		f.replaceValue(v, constOp)
		// remove left and right constant operands
		f.removeIfDead(left)
		f.removeIfDead(right)
	}
}

// evaluateBop applies op to two constant operands of type t
func evaluateBop(op Op, t *types.Type, left, right any) (any, bool) {
	switch {
	case types.Equal(t, types.Int()):
		leftVal := left.(int32)
		rightVal := right.(int32)

		switch op {
		case OpAdd:
			return leftVal + rightVal, true
		case OpSubtract:
			return leftVal - rightVal, true
		case OpMultiply:
			return leftVal * rightVal, true
		case OpDivide:
			if rightVal != 0 {
				return leftVal / rightVal, true
			}
		case OpEqual:
			return leftVal == rightVal, true
		case OpNotEqual:
			return leftVal != rightVal, true
		case OpLess:
			return leftVal < rightVal, true
		case OpLessEqual:
			return leftVal <= rightVal, true
		case OpGreater:
			return leftVal > rightVal, true
		case OpGreaterEqual:
			return leftVal >= rightVal, true
		}
	case types.Equal(t, types.Bool()):
		leftVal := left.(bool)
		rightVal := right.(bool)

		switch op {
		case OpEqual:
			return leftVal == rightVal, true
		case OpNotEqual:
			return leftVal != rightVal, true
		}
	}

	return nil, false
}
