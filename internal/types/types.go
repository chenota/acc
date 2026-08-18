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
	KPointer
	KTuple
)

type Type struct {
	kind Kind // making this private so outside callers are forced to use Equal.

	// for KFunction and KTuple
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

	// tuple comparison
	if a.kind == KTuple && b.kind == KTuple {
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

	// pointer comparison
	if a.kind == KPointer && b.kind == KPointer {
		return Equal(a.result, b.result)
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

		return fmt.Sprintf("fun (%s) -> %v", strings.Join(params, ","), t.result)
	case KTuple:
		elems := make([]string, len(t.params))
		for i := range t.params {
			elems[i] = t.params[i].String()
		}

		// a single element needs the trailing comma to read back as a tuple
		if len(elems) == 1 {
			return fmt.Sprintf("(%s,)", elems[0])
		}

		return fmt.Sprintf("(%s)", strings.Join(elems, ", "))
	case KPointer:
		return fmt.Sprintf("*%v", t.result)
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

func (t *Type) IsTuple() bool {
	if t == nil {
		return false
	}

	return t.kind == KTuple
}

func (t *Type) IsPointer() bool {
	if t == nil {
		return false
	}

	return t.kind == KPointer
}

func (t *Type) Params() []*Type {
	if t == nil {
		return nil
	}

	return t.params
}

func (t *Type) Result() *Type {
	if t == nil {
		return nil
	}

	return t.result
}

func (t *Type) IsUnit() bool {
	if t == nil {
		return false
	}

	return t.kind == KUnit
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

func Tuple(elems []*Type) *Type {
	return &Type{
		kind:   KTuple,
		params: elems,
	}
}

func Pointer(sub *Type) *Type {
	return &Type{
		kind:   KPointer,
		result: sub,
	}
}

// Size returns the type's size in bytes
func (t *Type) Size() int {
	switch t.kind {
	case KUnit:
		return 0
	case KInt:
		return 4
	case KTuple:
		// tuple layout does not pad or align elements yet
		size := 0
		for _, elem := range t.params {
			size += elem.Size()
		}
		return size
	default:
		return 8
	}
}

func (t *Type) ToDefault() *Type {
	switch {
	case Equal(t, UntypedInt()):
		return Int()
	case t.IsTuple():
		elems := make([]*Type, len(t.params))
		for i := range t.params {
			elems[i] = t.params[i].ToDefault()
		}
		return Tuple(elems)
	default:
		return t
	}
}

func (t *Type) IsScalar() bool {
	// every type but a tuple fits in a register right now
	return !t.IsTuple()
}
