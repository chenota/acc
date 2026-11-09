package ssa

import "github.com/chenota/acc/internal/target"

type Op int

const (
	OpUnknown Op = iota

	// generic ops, which mean the same thing on every target

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

	OpX64ADD
	OpX64SUB
	OpX64IMUL
	OpX64IDIV // signed divide of Args[0] by Args[1]
	OpX64NEG
	OpX64XORconst // xor of Args[0] with the int32 in Value

	OpARM64ADD
	OpARM64SUB
	OpARM64MUL
	OpARM64SDIV // signed divide of Args[0] by Args[1]
	OpARM64NEG
	OpARM64EORconst // xor of Args[0] with the int32 in Value
)

// opInfo is what the back end knows about an op
type opInfo struct {
	name string
	reg  regInfo
}

var (
	rax = target.NewMask(target.RegA)
	rdx = target.NewMask(target.RegD)
)

var opcodeTable = [...]opInfo{
	OpUnknown: {name: "Unknown"},

	OpLiteral:      {name: "Literal"},
	OpLoad:         {name: "Load"},
	OpStaticLoad:   {name: "StaticLoad"},
	OpStore:        {name: "Store"},
	OpStaticStore:  {name: "StaticStore"},
	OpAdd:          {name: "Add"},
	OpSubtract:     {name: "Subtract"},
	OpMultiply:     {name: "Multiply"},
	OpDivide:       {name: "Divide"},
	OpEqual:        {name: "Equal"},
	OpNotEqual:     {name: "NotEqual"},
	OpLess:         {name: "Less"},
	OpLessEqual:    {name: "LessEqual"},
	OpGreater:      {name: "Greater"},
	OpGreaterEqual: {name: "GreaterEqual"},
	OpNegate:       {name: "Negate"},
	OpNot:          {name: "Not"},
	OpCopy:         {name: "Copy"},
	OpStaticCall:   {name: "StaticCall"},
	OpParam:        {name: "Param"},
	OpResult:       {name: "Result"},
	OpCallResult:   {name: "CallResult"},
	OpLocalAddr:    {name: "LocalAddr"},
	OpFieldAddr:    {name: "FieldAddr"},
	OpClosurePtr:   {name: "ClosurePtr"},
	OpClosureCall:  {name: "ClosureCall"},
	OpUnit:         {name: "Unit"},
	OpLabelAddr:    {name: "LabelAddr"},
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
	OpX64NEG:      {name: "X64NEG", reg: regInfo{twoAddress: true}},
	OpX64XORconst: {name: "X64XORconst", reg: regInfo{twoAddress: true}},

	OpARM64ADD:      {name: "ARM64ADD"},
	OpARM64SUB:      {name: "ARM64SUB"},
	OpARM64MUL:      {name: "ARM64MUL"},
	OpARM64SDIV:     {name: "ARM64SDIV"},
	OpARM64NEG:      {name: "ARM64NEG"},
	OpARM64EORconst: {name: "ARM64EORconst"},
}

func (o Op) info() *opInfo {
	return &opcodeTable[o]
}

func (o Op) String() string {
	return o.info().name
}
