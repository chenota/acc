package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestType_String_Func(t *testing.T) {
	assert.Equal(t, "fun (int,int) -> int", Function([]*Type{Int(), Int()}, Int()).String())
}

func TestType_String_PointerToFunc(t *testing.T) {
	assert.Equal(t, "*fun () -> ()", Pointer(Function(nil, Unit())).String())
}

func TestType_String_Pointer(t *testing.T) {
	assert.Equal(t, "*int", Pointer(Int()).String())
}

func TestType_String_Tuple(t *testing.T) {
	assert.Equal(t, "(int, int)", Tuple([]*Type{Int(), Int()}).String())
}

func TestType_String_SingleElementTuple(t *testing.T) {
	assert.Equal(t, "(int,)", Tuple([]*Type{Int()}).String())
}

func TestEqual_Pointers(t *testing.T) {
	assert.True(t, Equal(Pointer(Int()), Pointer(Int())))
}

func TestEqual_Pointers_DifferentPointee(t *testing.T) {
	assert.False(t, Equal(Pointer(Int()), Pointer(Function(nil, Int()))))
}

func TestEqual_Pointers_DifferentDepth(t *testing.T) {
	assert.False(t, Equal(Pointer(Int()), Pointer(Pointer(Int()))))
}

func TestEqual_Pointer_NotPointer(t *testing.T) {
	assert.False(t, Equal(Pointer(Int()), Int()))
}
