package ssa

import (
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenSsa_Basic(t *testing.T) {
	inputStr := `fun main () -> int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	compiledFuncs, err := GenSsa(funcs)
	require.NoError(t, err)

	require.Len(t, compiledFuncs, 1)
	f := compiledFuncs[0]

	require.Len(t, f.Blocks, 1)
	b := f.Blocks[0]

	assert.Equal(t, BlockRet, b.Kind)

	require.NotNil(t, b.Control)
	assert.True(t, types.Equal(b.Control.Type, types.Int32()))
}
