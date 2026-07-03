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
	params []*Type
	result *Type
}

func Equal(a *Type, b *Type) bool {
	if a == nil || b == nil {
		return false
	}

	// function comparison
	if a.kind == KFunction && b.kind == KFunction {
		if !Equal(a.result, b.result) {
			return false
		}
		if len(a.params) != len(b.params) {
			return false
		}
		for i := range a.params {
			if !Equal(a.params[i], b.params[i]) {
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
		params := make([]string, len(t.params))
		for i := range t.params {
			params[i] = t.params[i].String()
		}

		return fmt.Sprintf("(%s) -> %v", strings.Join(params, ","), t.result)
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

func (t *Type) IsFunction() bool {
	if t == nil {
		return false
	}

	return t.kind == KFunction
}

func (t *Type) Params() []*Type {
	if t == nil || t.kind != KFunction {
		return nil
	}

	return t.params
}

func (t *Type) Result() *Type {
	if t == nil || t.kind != KFunction {
		return nil
	}

	return t.result
}

func Int() *Type {
	return &Type{kind: KInt}
}

func UntypedInt() *Type {
	return &Type{kind: KUntypedInt}
}

func Function(params []*Type, result *Type) *Type {
	return &Type{
		kind:   KFunction,
		params: params,
		result: result,
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

func (t *Type) IsScalar() bool {
	// every type is a scalar right now
	return true
}
