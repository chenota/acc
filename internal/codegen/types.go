package codegen

type Inst struct {
	Op   string
	Args []Arg
}

type ArgKind int

const (
	KRegister ArgKind = iota
	KImmediate
	KStack
)

type ArgSize int

const (
	Size8 ArgSize = iota
	Size16
	Size32
	Size64
)

type Arg struct {
	Kind   ArgKind
	Size   ArgSize
	AuxInt int64
}
