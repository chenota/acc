package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestType_String_Func(t *testing.T) {
	assert.Equal(t, "(int,int) -> int", Function([]*Type{Int(), Int()}, Int()).String())
}
