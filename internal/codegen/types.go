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

type Arg struct {
	Kind   ArgKind
	AuxInt int64
}
