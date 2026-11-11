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
	case OpEqual:
		v.Op = OpX64SETE
	case OpNotEqual:
		v.Op = OpX64SETNE
	case OpLess:
		v.Op = OpX64SETL
	case OpLessEqual:
		v.Op = OpX64SETLE
	case OpGreater:
		v.Op = OpX64SETG
	case OpGreaterEqual:
		v.Op = OpX64SETGE
	case OpLiteral:
		v.Op = OpX64MOVconst
	case OpLoad:
		v.Op = OpX64MOVload
	case OpStore:
		v.Op = OpX64MOVstore
	case OpLocalAddr:
		v.Op = OpX64LEAQslot
	case OpFieldAddr:
		v.Op = OpX64LEAQ
	case OpLabelAddr:
		v.Op = OpX64LEAQrip
	case OpStaticCall:
		v.Op = OpX64CALLstatic
	case OpClosureCall:
		v.Op = OpX64CALLclosure
	default:
		return false
	}
	return true
}
