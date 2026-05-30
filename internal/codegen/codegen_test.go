package codegen

import (
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/ssa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodegen_Basic(t *testing.T) {
	inputStr := `fun main () -> int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	ssaFuncs, err := ssa.BuildAndAllocate(funcs)
	require.NoError(t, err)

	insts := GenerateProgram(ssaFuncs)

	assertContainsOp(t, "_fmain:", insts)
	assertContainsOp(t, "ret", insts)
	assertContainsOpWithArg(t, "movl", KImmediate, insts)
}

func assertContainsOp(t *testing.T, op string, insts []Inst) {
	t.Helper()

	for _, inst := range insts {
		if inst.Op == op {
			return
		}
	}

	assert.Fail(t, "instructions list does not contain specified operation", "operation", op)
}

func assertContainsOpWithArg(t *testing.T, op string, arg ArgKind, insts []Inst) {
	t.Helper()

	for _, inst := range insts {
		if inst.Op == op {
			for _, a := range inst.Args {
				if a.Kind == arg {
					return
				}
			}
		}
	}

	assert.Fail(t, "instructions list does not contain specified operation with argument", "operation", op, "arg", arg)
}
