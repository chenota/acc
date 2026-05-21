package types

type Type interface {
	isType()
}

type IntSize int

const (
	IntSizeUnknown IntSize = iota
	IntSize8
	IntSize16
	IntSize32
	IntSize64
)

type Int struct {
	Size IntSize
}

func (i Int) isType() {}

type Function struct {
	Inputs []Type
	Output Type
}

func (f Function) isType() {}

type Unit struct{}

func (u Unit) isType() {}
