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
	leaves, err := Int().Leaves()

	assert.NoError(t, err)
	assert.Equal(t, []int{0}, leafOffsets(leaves))
	assert.True(t, Equal(Int(), leaves[0].Type))
}

func TestLeaves_Tuple(t *testing.T) {
	leaves, err := Tuple([]*Type{Int(), Int()}).Leaves()

	assert.NoError(t, err)
	assert.Equal(t, []int{0, 4}, leafOffsets(leaves))
}

func TestLeaves_TupleSkipsPadding(t *testing.T) {
	// an int followed by a pointer pads bytes 4-7 to align the pointer
	leaves, err := Tuple([]*Type{Int(), Pointer(Int())}).Leaves()

	assert.NoError(t, err)
	assert.Equal(t, []int{0, 8}, leafOffsets(leaves))
}

func TestLeaves_NestedTuple(t *testing.T) {
	leaves, err := Tuple([]*Type{Tuple([]*Type{Int(), Int()}), Int()}).Leaves()

	assert.NoError(t, err)
	assert.Equal(t, []int{0, 4, 8}, leafOffsets(leaves))
}

func TestLeaves_UnitField(t *testing.T) {
	leaves, err := Tuple([]*Type{Int(), Unit()}).Leaves()

	assert.NoError(t, err)
	assert.Equal(t, []int{0}, leafOffsets(leaves))
}

func leafOffsets(leaves []Leaf) []int {
	offsets := make([]int, len(leaves))
	for i, l := range leaves {
		offsets[i] = l.Offset
	}
	return offsets
}
