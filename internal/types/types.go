package types

type Kind int

const (
	KUnknown Kind = iota // important that unknown is the zero value
	KUnit
	KUntypedInt
	KInt32
	KFunction
	KMem
)

type Type struct {
	Kind Kind

	// for KFunction
	Inputs []*Type
	Output *Type
}

func Equal(a *Type, b *Type) bool {
	if a == nil || b == nil {
		return false
	}

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

	// atom comparison: just use the kinds
	return a.Kind == b.Kind
}

func Int32() *Type {
	return &Type{Kind: KInt32}
}

func UntypedInt() *Type {
	return &Type{Kind: KUntypedInt}
}

func Function(inputs []*Type, output *Type) *Type {
	return &Type{
		Kind:   KFunction,
		Inputs: inputs,
		Output: output,
	}
}

func Mem() *Type {
	return &Type{Kind: KMem}
}

func (t *Type) Size() int {
	switch t.Kind {
	case KUnknown, KUntypedInt, KMem:
		return -1
	case KUnit:
		return 0
	case KInt32:
		return 32
	default:
		return 64
	}
}
