package codegen

import (
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/register"
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

func TestCodegen_MainWrapper(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, insts, "call", "movq", "movq", "syscall")
}

func TestCodegen_Directives(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, insts, ".text", ".globl")
}

func TestCodegen_Call_TargetsCalleeLabel(t *testing.T) {
	insts := requireGeneratesProgram(t, `
		fun target (a int) -> int { return 0; }
		fun main () -> int { return target(7); }
	`)

	// the call must reference the callee's mangled label, not its source name
	assertCallsLabel(t, insts, "_target")
}

func TestCodegen_Call_ArgsInRegisters(t *testing.T) {
	insts := requireGeneratesProgram(t, `
		fun target (a int, b int, c int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3); }
	`)

	assertWritesRegBeforeCall(t, insts, register.RegDI)
	assertWritesRegBeforeCall(t, insts, register.RegSI)
	assertWritesRegBeforeCall(t, insts, register.RegD)
}

func TestCodegen_RedundantMoves(t *testing.T) {
	insts := requireGeneratesProgram(t, `
		fun double (x int) -> int { return x * 2; }
		fun main () -> int { return double(4); }
	`)

	assertNoSelfMoves(t, insts)
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

func assertCallsLabel(t *testing.T, insts []Inst, label string) {
	t.Helper()
	for _, inst := range insts {
		if inst.Op == "call" && inst.Dest.Kind == KText && inst.Dest.Value == label {
			return
		}
	}
	assert.Fail(t, "instructions list does not call the specified label", label)
}

func assertWritesRegBeforeCall(t *testing.T, insts []Inst, reg register.Register) {
	t.Helper()
	var written bool
	for _, inst := range insts {
		if strings.HasPrefix(inst.Op, "mov") && inst.Dest.Kind == KRegister && inst.Dest.Reg == reg {
			written = true
		}
		if inst.Op == "call" && written {
			return
		}
	}
	assert.Fail(t, "no mov writes the register before a call", "reg=%d", reg)
}

func assertNoSelfMoves(t *testing.T, insts []Inst) {
	t.Helper()
	for _, inst := range insts {
		if strings.HasPrefix(inst.Op, "mov") && inst.Src1.Kind != KUndefined && inst.Src1 == inst.Dest {
			assert.Fail(t, "a mov onto its own location survived codegen", "%+v", inst)
		}
	}
}

func requireGeneratesProgram(t *testing.T, src string) []Inst {
	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	ssaFuncs, err := ssa.BuildAndAllocate(funcs)
	require.NoError(t, err)

	p, err := GenerateProgram(ssaFuncs)
	require.NoError(t, err)

	return p
}
