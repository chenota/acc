package types

import (
	"fmt"
	"strings"
)

type Kind int

const (
	KUnknown Kind = iota // important that unknown is the zero value
	KUnit
	KUntypedInt
	KInt
	KFunction
)

type Type struct {
	kind Kind // making this private so outside callers are forced to use Equal.

	// for KFunction
	Inputs []*Type
	Output *Type
}

func Equal(a *Type, b *Type) bool {
	if a == nil || b == nil {
		return false
	}

	// function comparison
	if a.kind == KFunction && b.kind == KFunction {
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
	return a.kind == b.kind
}

func (t *Type) IsConcreteNumeric() bool {
	if t == nil {
		return false
	}

	return t.kind == KInt
}

func (t *Type) String() string {
	switch t.kind {
	case KUnit:
		return "()"
	case KUntypedInt:
		return "untyped int"
	case KInt:
		return "int"
	case KFunction:
		params := make([]string, len(t.Inputs))
		for i := range t.Inputs {
			params[i] = t.Inputs[i].String()
		}

		return fmt.Sprintf("(%s) -> %v", strings.Join(params, ","), t.Output)
	default:
		return "unknown"
	}
}

func (t *Type) IsUntypedNumeric() bool {
	if t == nil {
		return false
	}

	return t.kind == KUntypedInt
}

func Int() *Type {
	return &Type{kind: KInt}
}

func UntypedInt() *Type {
	return &Type{kind: KUntypedInt}
}

func Function(inputs []*Type, output *Type) *Type {
	return &Type{
		kind:   KFunction,
		Inputs: inputs,
		Output: output,
	}
}

func Unit() *Type {
	return &Type{kind: KUnit}
}

// Size returns the type's size in bytes
func (t *Type) Size() int {
	switch t.kind {
	case KUnit:
		return 0
	case KInt:
		return 4
	default:
		return 8
	}
}

func (t *Type) ToDefault() *Type {
	switch {
	case t == nil:
		return Unit()
	case Equal(t, UntypedInt()):
		return Int()
	default:
		return t
	}
}
