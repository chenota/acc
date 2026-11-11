package ssa

import "github.com/chenota/acc/internal/target"

type Op int

const (
	OpUnknown Op = iota

	// high-level and target-agnostic ops

	OpLiteral
	OpLoad
	OpStaticLoad // reads the frame slot named in Value
	OpStore
	OpStaticStore // writes Args[0] to the frame slot named in Value
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
	OpEqual
	OpNotEqual
	OpLess
	OpLessEqual
	OpGreater
	OpGreaterEqual
	OpNegate
	OpNot
	OpCopy
	OpStaticCall  // direct call to the function in Value
	OpParam       // incoming function argument - more of a placeholder for a location than an acutal value in its own right
	OpResult      // outgoing function result - the mirror of OpParam
	OpCallResult  // incoming call result
	OpLocalAddr   // address bound to a static stack slot
	OpFieldAddr   // address at Offset from the pointer in Args[0]
	OpClosurePtr  // incoming environment pointer the caller left in the context register
	OpClosureCall // indirect call to the code pointer in Args[0], with its environment in Args[1] and ABI arguments after
	OpUnit
	OpLabelAddr // address bound to a label
	OpPhi

	// x64 ops

	OpX64ADD
	OpX64SUB
	OpX64IMUL
	OpX64IDIV // signed divide of Args[0] by Args[1]
	OpX64NEG
	OpX64XORconst // xor of Args[0] with the int32 in Value
	OpX64SETE     // compares Args[0] with Args[1], true when equal
	OpX64SETNE    // compares Args[0] with Args[1], true when not equal
	OpX64SETL     // compares Args[0] with Args[1], true when signed less
	OpX64SETLE    // compares Args[0] with Args[1], true when signed less or equal
	OpX64SETG     // compares Args[0] with Args[1], true when signed greater
	OpX64SETGE    // compares Args[0] with Args[1], true when signed greater or equal
	OpX64MOVconst // the constant in Value
	OpX64MOVload  // reads from Offset past the pointer in Args[0]
	OpX64MOVstore // writes Args[0] to Offset past the pointer in Args[1]
	OpX64LEAQslot // address at Offset into the frame slot in Value
	OpX64LEAQ     // address at Offset from the pointer in Args[0]
	OpX64LEAQrip  // address of the function in Value
	OpX64CALLstatic
	OpX64CALLclosure

	// arm64 ops

	OpARM64ADD
	OpARM64SUB
	OpARM64MUL
	OpARM64SDIV // signed divide of Args[0] by Args[1]
	OpARM64NEG
	OpARM64EORconst // xor of Args[0] with the int32 in Value
	OpARM64CSETEQ   // compares Args[0] with Args[1], true when equal
	OpARM64CSETNE   // compares Args[0] with Args[1], true when not equal
	OpARM64CSETLT   // compares Args[0] with Args[1], true when signed less
	OpARM64CSETLE   // compares Args[0] with Args[1], true when signed less or equal
	OpARM64CSETGT   // compares Args[0] with Args[1], true when signed greater
	OpARM64CSETGE   // compares Args[0] with Args[1], true when signed greater or equal
	OpARM64MOVconst // the constant in Value
	OpARM64LDR      // reads from Offset past the pointer in Args[0]
	OpARM64STR      // writes Args[0] to Offset past the pointer in Args[1]
	OpARM64ADDslot  // address at Offset into the frame slot in Value
	OpARM64ADDconst // address at Offset from the pointer in Args[0]
	OpARM64ADRP     // address of the function in Value, an adrp of its page plus an add of the rest
	OpARM64BL
	OpARM64BLR
)

// opInfo is a collection of properties of an operation
type opInfo struct {
	name string
	reg  regInfo

	call             bool
	closureCall      bool // code pointer and environment come before its arguments
	rematerializable bool // can be recomputed rather than being spilled
	noResult         bool

	constant    bool
	unary       bool
	binary      bool
	comparison  bool
	associative bool
}

var (
	rax = target.NewMask(target.RegA)
	rdx = target.NewMask(target.RegD)
)

var opcodeTable = [...]opInfo{
	OpUnknown: {name: "Unknown"},

	OpLiteral:      {name: "Literal", rematerializable: true, constant: true},
	OpLoad:         {name: "Load"},
	OpStaticLoad:   {name: "StaticLoad"},
	OpStore:        {name: "Store", noResult: true},
	OpStaticStore:  {name: "StaticStore", noResult: true},
	OpAdd:          {name: "Add", binary: true, associative: true},
	OpSubtract:     {name: "Subtract", binary: true},
	OpMultiply:     {name: "Multiply", binary: true, associative: true},
	OpDivide:       {name: "Divide", binary: true},
	OpEqual:        {name: "Equal", comparison: true},
	OpNotEqual:     {name: "NotEqual", comparison: true},
	OpLess:         {name: "Less", comparison: true},
	OpLessEqual:    {name: "LessEqual", comparison: true},
	OpGreater:      {name: "Greater", comparison: true},
	OpGreaterEqual: {name: "GreaterEqual", comparison: true},
	OpNegate:       {name: "Negate", unary: true},
	OpNot:          {name: "Not", unary: true},
	OpCopy:         {name: "Copy"},
	OpStaticCall:   {name: "StaticCall", call: true, noResult: true},
	OpParam:        {name: "Param"},
	OpResult:       {name: "Result", noResult: true},
	OpCallResult:   {name: "CallResult"},
	OpLocalAddr:    {name: "LocalAddr", rematerializable: true},
	OpFieldAddr:    {name: "FieldAddr"},
	OpClosurePtr:   {name: "ClosurePtr"},
	OpClosureCall:  {name: "ClosureCall", call: true, closureCall: true, noResult: true},
	OpUnit:         {name: "Unit"},
	OpLabelAddr:    {name: "LabelAddr", rematerializable: true},
	OpPhi:          {name: "Phi"},

	OpX64ADD:  {name: "X64ADD", reg: regInfo{twoAddress: true}},
	OpX64SUB:  {name: "X64SUB", reg: regInfo{twoAddress: true}},
	OpX64IMUL: {name: "X64IMUL", reg: regInfo{twoAddress: true}},
	/*
		NOTE TO SELF:

		idiv divides rdx:rax by the divisor (arbitrary register), and leaves the quotient in rax and the remainder in rdx.
		In codegen, this makes it important to sign-extend rax into rdx before dividing.
	*/
	OpX64IDIV: {name: "X64IDIV", reg: regInfo{
		inputs:   []target.Mask{rax, anyReg.Without(rax.Union(rdx))},
		output:   rax,
		clobbers: rdx,
	}},
	OpX64NEG:         {name: "X64NEG", reg: regInfo{twoAddress: true}},
	OpX64XORconst:    {name: "X64XORconst", reg: regInfo{twoAddress: true}},
	OpX64SETE:        {name: "X64SETE"},
	OpX64SETNE:       {name: "X64SETNE"},
	OpX64SETL:        {name: "X64SETL"},
	OpX64SETLE:       {name: "X64SETLE"},
	OpX64SETG:        {name: "X64SETG"},
	OpX64SETGE:       {name: "X64SETGE"},
	OpX64MOVconst:    {name: "X64MOVconst", rematerializable: true, constant: true},
	OpX64MOVload:     {name: "X64MOVload"},
	OpX64MOVstore:    {name: "X64MOVstore", noResult: true},
	OpX64LEAQslot:    {name: "X64LEAQslot", rematerializable: true},
	OpX64LEAQ:        {name: "X64LEAQ"},
	OpX64LEAQrip:     {name: "X64LEAQrip", rematerializable: true},
	OpX64CALLstatic:  {name: "X64CALLstatic", call: true, noResult: true},
	OpX64CALLclosure: {name: "X64CALLclosure", call: true, closureCall: true, noResult: true},

	OpARM64ADD:      {name: "ARM64ADD"},
	OpARM64SUB:      {name: "ARM64SUB"},
	OpARM64MUL:      {name: "ARM64MUL"},
	OpARM64SDIV:     {name: "ARM64SDIV"},
	OpARM64NEG:      {name: "ARM64NEG"},
	OpARM64EORconst: {name: "ARM64EORconst"},
	OpARM64CSETEQ:   {name: "ARM64CSETEQ"},
	OpARM64CSETNE:   {name: "ARM64CSETNE"},
	OpARM64CSETLT:   {name: "ARM64CSETLT"},
	OpARM64CSETLE:   {name: "ARM64CSETLE"},
	OpARM64CSETGT:   {name: "ARM64CSETGT"},
	OpARM64CSETGE:   {name: "ARM64CSETGE"},
	OpARM64MOVconst: {name: "ARM64MOVconst", rematerializable: true, constant: true},
	OpARM64LDR:      {name: "ARM64LDR"},
	OpARM64STR:      {name: "ARM64STR", noResult: true},
	OpARM64ADDslot:  {name: "ARM64ADDslot", rematerializable: true},
	OpARM64ADDconst: {name: "ARM64ADDconst"},
	OpARM64ADRP:     {name: "ARM64ADRP", rematerializable: true},
	OpARM64BL:       {name: "ARM64BL", call: true, noResult: true},
	OpARM64BLR:      {name: "ARM64BLR", call: true, closureCall: true, noResult: true},
}

func (o Op) info() *opInfo {
	return &opcodeTable[o]
}

func (o Op) String() string {
	return o.info().name
}
