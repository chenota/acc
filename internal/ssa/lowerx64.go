package ssa

func lowerValueX64(v *Value) bool {
	switch v.Op {
	case OpAdd:
		v.Op = OpX64ADD
	case OpSubtract:
		v.Op = OpX64SUB
	case OpMultiply:
		v.Op = OpX64IMUL
	case OpDivide:
		v.Op = OpX64IDIV
	case OpNegate:
		v.Op = OpX64NEG
	case OpNot:
		v.Op = OpX64XORconst
		v.Value = int32(1)
	default:
		return false
	}
	return true
}
