package ssa

import "github.com/chenota/acc/internal/types"

func foldConstants(f *Func) {
	for _, b := range f.OrderedBlocks() {
		for _, v := range b.OrderedValues() {
			// exclude everything that isn't a bop with constant args; this will need expanded later
			if !v.IsBinaryOp() || !v.Args[0].IsConstant() || !v.Args[1].IsConstant() {
				continue
			}

			foldedVal, ok := evaluateBop(v.Op, v.Type, v.Args[0].Value, v.Args[1].Value)
			if !ok {
				continue
			}

			constOp := f.newValue(OpLiteral, v.Type, b)
			constOp.Value = foldedVal

			f.substituteValue(v, constOp)
		}
	}
}

func evaluateBop(op Op, t *types.Type, left, right any) (any, bool) {
	switch {
	case types.Equal(t, types.Int32()):
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
		}
	}

	return nil, false
}
