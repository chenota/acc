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

	// Function prologue/epilogue
	assertContainsSeq(t, insts, "pushq", "movq", "popq", "ret")
	// Immediate
	assertContainsOpWithArgs(t, insts, "movl", KImmediate, KRegister)
}

func assertContainsSeq(t *testing.T, insts []Inst, seq ...string) {
	t.Helper()

	var seqIdx int
	for _, inst := range insts {
		if inst.Op == seq[seqIdx] {
			seqIdx += 1
		}
		if seqIdx >= len(seq) {
			return
		}
	}

	assert.Fail(t, "instructions list does not contain the specified sequence of operations")
}

func assertContainsOpWithArgs(t *testing.T, insts []Inst, op string, args ...ArgKind) {
	t.Helper()
	for _, inst := range insts {
		if inst.Op == op && len(inst.Args) == len(args) {
			for i := range args {
				if inst.Args[i].Kind != args[i] {
					break
				}
				if i == len(args)-1 {
					return
				}
			}
		}
	}
	assert.Fail(t, "instructions list does not contain specified operation with argument", "operation", op, "args", args)
}
