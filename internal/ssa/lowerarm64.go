package ssa

func lowerValueARM64(v *Value) bool {
	switch v.Op {
	case OpAdd:
		v.Op = OpARM64ADD
	case OpSubtract:
		v.Op = OpARM64SUB
	case OpMultiply:
		v.Op = OpARM64MUL
	case OpDivide:
		v.Op = OpARM64SDIV
	case OpNegate:
		v.Op = OpARM64NEG
	case OpNot:
		v.Op = OpARM64EORconst
		v.Value = int32(1)
	case OpEqual:
		v.Op = OpARM64CSETEQ
	case OpNotEqual:
		v.Op = OpARM64CSETNE
	case OpLess:
		v.Op = OpARM64CSETLT
	case OpLessEqual:
		v.Op = OpARM64CSETLE
	case OpGreater:
		v.Op = OpARM64CSETGT
	case OpGreaterEqual:
		v.Op = OpARM64CSETGE
	default:
		return false
	}
	return true
}
