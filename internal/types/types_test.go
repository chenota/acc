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

func TestLeaves_Scalar(t *testing.T) {
	assert.Equal(t, []int{0}, leafOffsets(Int()))
}

func TestLeaves_Tuple(t *testing.T) {
	assert.Equal(t, []int{0, 4}, leafOffsets(Tuple([]*Type{Int(), Int()})))
}

func TestLeaves_TupleSkipsPadding(t *testing.T) {
	// an int followed by a pointer pads bytes 4-7 to align the pointer
	assert.Equal(t, []int{0, 8}, leafOffsets(Tuple([]*Type{Int(), Pointer(Int())})))
}

func TestLeaves_NestedTuple(t *testing.T) {
	assert.Equal(t, []int{0, 4, 8}, leafOffsets(Tuple([]*Type{Tuple([]*Type{Int(), Int()}), Int()})))
}

func TestLeaves_UnitField(t *testing.T) {
	assert.Equal(t, []int{0}, leafOffsets(Tuple([]*Type{Int(), Unit()})))
}

func TestIsSingleton_Scalar(t *testing.T) {
	assert.False(t, Int().IsSingleton())
}

func TestIsSingleton_Unit(t *testing.T) {
	assert.True(t, Unit().IsSingleton())
}

func TestIsSingleton_TupleOfSingletons(t *testing.T) {
	assert.True(t, Tuple([]*Type{Unit(), Unit()}).IsSingleton())
}

func TestLeaves_Unit(t *testing.T) {
	assert.Empty(t, leafOffsets(Unit()))
}

func leafOffsets(t *Type) []int {
	var offsets []int
	for offset := range t.Leaves() {
		offsets = append(offsets, offset)
	}
	return offsets
}
