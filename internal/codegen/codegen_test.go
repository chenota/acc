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

func TestCodegen_Directives(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, insts, ".text", ".globl")
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

func TestCodegen_CalleeSavedSavedBeforeStackAdjust(t *testing.T) {
	// caller keeps `a` in a callee-saved register across a call that passes arguments on the stack.
	insts := requireGeneratesProgram(t, `
		fun sink (x1 int, x2 int, x3 int, x4 int, x5 int, x6 int, x7 int, x8 int) -> int { return 0; }
		fun caller (a int) -> int { return sink(1, 2, 3, 4, 5, 6, 7, 8) + a; }
		fun main () -> int { return caller(0); }
	`)

	assertSavesCalleeBeforeStackAdjust(t, insts)
}

func TestCodegen_LocalAddr_LeaqSlotIntoRegister(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	lea := requireOnlyOp(t, insts, "leaq")

	// the frame slot is the source and a register receives the address it forms
	assert.Equal(t, KMemory, lea.Src1.Kind)
	assert.Equal(t, register.RegBP, lea.Src1.Reg)
	assert.Equal(t, KRegister, lea.Dest.Kind)
}

func TestCodegen_AddressedLocal_LivesInFrameSlot(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	// taking a's address keeps it in memory, so the initializer is written to its slot
	assertContainsMemDest(t, insts, "movl", register.RegBP, -4)

	// and the prologue reserves room for the slot
	assertContainsOpWithArgs(t, insts, "subq", KImmediate, KUndefined, KRegister)
}

func TestCodegen_Deref_LoadsThroughAddressRegister(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	lea := requireOnlyOp(t, insts, "leaq")
	require.Equal(t, KRegister, lea.Dest.Kind)

	// the deref reads the memory the address register points at, not the register itself
	assertContainsMemSrc(t, insts, "movl", lea.Dest.Reg, 0)
}

func TestCodegen_DerefAssignment_StoresThroughAddressRegister(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { let a = 10; let b = &a; *b = 20; return a; }`)

	lea := requireOnlyOp(t, insts, "leaq")
	require.Equal(t, KRegister, lea.Dest.Kind)

	assertContainsMemDest(t, insts, "movl", lea.Dest.Reg, 0)
}

func TestCodegen_DerefAssignOp_FormsAddressOnce(t *testing.T) {
	insts := requireGeneratesProgram(t, `fun main () -> int { let a = 10; let b = &a; *b += 5; return a; }`)

	// *b += 5 reads and writes through one address, so only one leaq is emitted
	lea := requireOnlyOp(t, insts, "leaq")
	require.Equal(t, KRegister, lea.Dest.Kind)

	assertContainsMemSrc(t, insts, "movl", lea.Dest.Reg, 0)
	assertContainsMemDest(t, insts, "movl", lea.Dest.Reg, 0)
}

func TestCodegen_EscapingLocal_CallsRuntimeAllocator(t *testing.T) {
	insts := requireGeneratesProgram(t, `
		fun escape () -> *int { let a = 10; return &a; }
		fun main () -> int { let p = escape(); return *p; }
	`)

	// the allocator is an ordinary call to a symbol the C runtime defines
	assertContainsCallTo(t, insts, symbol("acc_alloc"))
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

// requireOnlyOp returns the sole instruction using the given mnemonic.
func requireOnlyOp(t *testing.T, insts []Inst, op string) Inst {
	t.Helper()
	var found []Inst
	for _, inst := range insts {
		if inst.Op == op {
			found = append(found, inst)
		}
	}
	require.Len(t, found, 1, "expected exactly one %q instruction", op)
	return found[0]
}

// assertContainsMemSrc looks for an instruction reading memory at offset(reg).
func assertContainsMemSrc(t *testing.T, insts []Inst, op string, reg register.Register, offset int) {
	t.Helper()
	for _, inst := range insts {
		if inst.Op == op && isMem(inst.Src1, reg, offset) {
			return
		}
	}
	assert.Fail(t, "no instruction reads the specified memory operand", "%s %d(%v), _", op, offset, reg)
}

// assertContainsMemDest looks for an instruction writing memory at offset(reg).
func assertContainsMemDest(t *testing.T, insts []Inst, op string, reg register.Register, offset int) {
	t.Helper()
	for _, inst := range insts {
		if inst.Op == op && isMem(inst.Dest, reg, offset) {
			return
		}
	}
	assert.Fail(t, "no instruction writes the specified memory operand", "%s _, %d(%v)", op, offset, reg)
}

// assertContainsCallTo looks for a call to the named symbol.
func assertContainsCallTo(t *testing.T, insts []Inst, sym string) {
	t.Helper()
	for _, inst := range insts {
		if inst.Op == "call" && inst.Dest.Kind == KText && inst.Dest.Value == sym {
			return
		}
	}
	assert.Fail(t, "no instruction calls the specified symbol", sym)
}

// isMem reports whether arg addresses memory at offset(reg).
func isMem(arg Arg, reg register.Register, offset int) bool {
	return arg.Kind == KMemory && arg.Reg == reg && arg.Value == offset
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

func assertSavesCalleeBeforeStackAdjust(t *testing.T, insts []Inst) {
	t.Helper()
	firstPush, firstSub := -1, -1
	for i, inst := range insts {
		if firstPush < 0 && inst.Op == "pushq" && inst.Dest.Kind == KRegister && register.CalleeSaved.Contains(inst.Dest.Reg) {
			firstPush = i
		}
		if firstSub < 0 && inst.Op == "subq" && inst.Dest.Kind == KRegister && inst.Dest.Reg == register.RegSP {
			firstSub = i
		}
	}
	require.NotEqual(t, -1, firstPush, "expected a callee-saved register to be pushed")
	require.NotEqual(t, -1, firstSub, "expected a stack pointer adjustment")
	assert.Less(t, firstPush, firstSub, "callee-saved pushes must precede the stack adjustment")
}

func requireGeneratesProgram(t *testing.T, src string) []Inst {
	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	allFuncs, err := semantic.Analyze(funcs)
	require.NoError(t, err)

	ssaFuncs, err := ssa.BuildAndAllocate(allFuncs)
	require.NoError(t, err)

	p, err := GenerateProgram(ssaFuncs)
	require.NoError(t, err)

	return p
}
