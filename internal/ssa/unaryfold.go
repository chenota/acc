package ssa

import "github.com/chenota/acc/internal/types"

func unaryFold(f *Func) {
	for _, v := range f.values() {
		if !v.IsUnaryOp() {
			continue
		}

		sub := v.Args[0]

		if !sub.IsConstant() {
			continue
		}

		foldedVal, ok := evaluateUop(v.Op, v.Type, sub.Value)
		if !ok {
			continue
		}

		constOp := f.newValue(OpLiteral, v.Type, v.Block)
		constOp.Value = foldedVal

		// replace the top-level value with the new constant
		f.substituteValue(v, constOp)
		// remove sub value
		f.removeValue(sub)
	}
}

func evaluateUop(op Op, t *types.Type, sub any) (any, bool) {
	switch {
	case types.Equal(t, types.Int()):
		val := sub.(int32)

		switch op {
		case OpNegate:
			return -val, true
		}
	}

	return nil, false
}
