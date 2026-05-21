package types

type Kind int

const (
	KUnknown Kind = iota // important that unknown is the zero value
	KUnit
	KUntypedInt
	KInt32
	KFunction
)

type Type struct {
	Kind Kind

	// for KFunction
	Inputs []*Type
	Output *Type
}
