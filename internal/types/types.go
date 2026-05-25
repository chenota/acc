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

func Equal(a *Type, b *Type) bool {
	// function comparison
	if a.Kind == KFunction && b.Kind == KFunction {
		if !Equal(a.Output, b.Output) {
			return false
		}
		if len(a.Inputs) != len(b.Inputs) {
			return false
		}
		for i := range a.Inputs {
			if !Equal(a.Inputs[i], b.Inputs[i]) {
				return false
			}
		}
		return true
	}

	// atom comparison: use direct pointer comparison
	return a == b
}
