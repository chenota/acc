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

func TestCodegen_PrologueEpilogue(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, insts, "pushq", "movq", "popq", "ret")
}

func TestCodegen_ImmediateValue(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsOpWithArgs(t, insts, "movl", KImmediate, KUndefined, KRegister)
}

func TestCodegen_Directives(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, insts, ".text", ".globl")
}

func TestCodegen_Metadata(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, insts, ".type", ".size")
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

	assert.Fail(t, "instructions list does not contain the specified sequence of operations", seq)
}

func assertContainsOpWithArgs(t *testing.T, insts []Inst, op string, src1, src2, dest ArgKind) {
	t.Helper()
	for _, inst := range insts {
		if inst.Op == op && inst.Src1.Kind == src1 && inst.Src2.Kind == src2 && inst.Dest.Kind == dest {
			return
		}
	}
	assert.Fail(t, "instructions list does not contain specified operation with arguments", op, src1, src2, dest)
}

func requireGeneratesProgram(t *testing.T, src string) []Inst {
	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	ssaFuncs, err := ssa.BuildAndAllocate(funcs)
	require.NoError(t, err)

	return GenerateProgram(ssaFuncs)
}
