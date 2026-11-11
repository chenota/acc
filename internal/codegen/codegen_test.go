package codegen

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/chenota/acc/internal/buildcfg"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/ssa"
	"github.com/chenota/acc/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodegen_PrologueEpilogue(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { return 0; }`)

	assertContainsSeq(t, asm, "pushq %rbp", "movq %rsp, %rbp", "popq %rbp", "ret")
}

func TestCodegen_Labels_ELF(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { return 0; }`)

	assert.Contains(t, asm, "\t.globl main\n")
	assert.Equal(t, []string{"main", ".Lmain.0", ".Lmain.ret"}, labels(asm))
}

func TestCodegen_Labels_MachO(t *testing.T) {
	asm := requireGeneratesAssemblyFor(t, buildcfg.OSDarwin, `fun main () -> int { return 0; }`)

	assert.Contains(t, asm, "\t.globl _main\n")
	assert.Equal(t, []string{"_main", "Lmain.0", "Lmain.ret"}, labels(asm))
}

func TestCodegen_ImmediateValue(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { return 0; }`)

	assert.Contains(t, instructions(asm), "movl $0, %eax")
}

func TestGenerateAssembly_Directives(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { return 0; }`)

	assert.True(t, strings.HasPrefix(asm, "\t.text\n\t.globl main\nmain:\n"))
}

func TestGenerateAssembly_NoGeneratorForArch(t *testing.T) {
	funcs := requireBuildSSAFor(t, target.ARM64AAPCS, `fun main () -> int { return 0; }`)

	_, err := GenerateAssembly(funcs, WithTarget(buildcfg.ArchARM64, buildcfg.OSLinux))

	assert.ErrorContains(t, err, "no code generator for arm64")
}

func TestCodegen_Call_ArgsInRegisters(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun target (a int, b int, c int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3); }
	`)

	assertContainsSeq(t, asm, "movl $1, %edi", "call target")
	assertContainsSeq(t, asm, "movl $2, %esi", "call target")
	assertContainsSeq(t, asm, "movl $3, %edx", "call target")
}

func TestCodegen_RedundantMoves(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun double (x int) -> int { return x * 2; }
		fun main () -> int { return double(4); }
	`)

	assertNoSelfMoves(t, asm)
}

func TestCodegen_Divide_TrapIsLaidOutAfterTheDivide(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun div (a int, b int) -> int { return a / b; }
		fun main () -> int { return div(7, 2); }
	`)

	stop := requireOnly(t, asm, "ud2")
	div := requireOnly(t, asm, "idivl")

	// a nonzero divisor runs straight on into the divide, and only a zero one jumps away to stop the program
	assertContainsSeq(t, asm, div, stop)
}

func TestCodegen_CalleeSavedSavedBeforeStackAdjust(t *testing.T) {
	// caller keeps `a` in a callee-saved register across a call that passes arguments on the stack.
	asm := requireGeneratesAssembly(t, `
		fun sink (x1 int, x2 int, x3 int, x4 int, x5 int, x6 int, x7 int, x8 int) -> int { return 0; }
		fun caller (a int) -> int { return sink(1, 2, 3, 4, 5, 6, 7, 8) + a; }
		fun main () -> int { return caller(0); }
	`)

	push := indexOfMatch(asm, `^pushq %(rbx|r12|r13|r14|r15)$`)
	sub := indexOfMatch(asm, `^subq \$\d+, %rsp$`)

	require.NotEqual(t, -1, push, "expected a callee-saved register to be pushed")
	require.NotEqual(t, -1, sub, "expected a stack pointer adjustment")
	assert.Less(t, push, sub, "callee-saved pushes must precede the stack adjustment")
}

func TestCodegen_LocalAddr_LeaqSlotIntoRegister(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	lea := operands(requireOnly(t, asm, "leaq"))
	require.Len(t, lea, 2)

	// the frame slot is the source and a register receives the address it forms
	assert.Regexp(t, `^-\d+\(%rbp\)$`, lea[0])
	assert.Regexp(t, `^`+quadRegister+`$`, lea[1])
}

func TestCodegen_AddressedLocal_LivesInFrameSlot(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	// taking a's address keeps it in memory, so the initializer is written to its slot
	assertContainsMatch(t, asm, `^movl \S+, -4\(%rbp\)$`)

	// and the prologue reserves room for the slot
	assertContainsMatch(t, asm, `^subq \$\d+, %rsp$`)
}

func TestCodegen_Deref_LoadsThroughAddressRegister(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	lea := operands(requireOnly(t, asm, "leaq"))
	require.Len(t, lea, 2)

	// the deref reads the memory the address register points at, not the register itself
	assertContainsMatch(t, asm, `^movl 0\(`+regexp.QuoteMeta(lea[1])+`\), \S+$`)
}

func TestCodegen_DerefAssignment_StoresThroughAddressRegister(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { let a = 10; let b = &a; *b = 20; return a; }`)

	lea := operands(requireOnly(t, asm, "leaq"))
	require.Len(t, lea, 2)

	assertContainsMatch(t, asm, `^movl \S+, 0\(`+regexp.QuoteMeta(lea[1])+`\)$`)
}

func TestCodegen_DerefAssignOp_FormsAddressOnce(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { let a = 10; let b = &a; *b += 5; return a; }`)

	// *b += 5 reads and writes through one address, so only one leaq is emitted
	lea := operands(requireOnly(t, asm, "leaq"))
	require.Len(t, lea, 2)

	assertContainsMatch(t, asm, `^movl 0\(`+regexp.QuoteMeta(lea[1])+`\), \S+$`)
	assertContainsMatch(t, asm, `^movl \S+, 0\(`+regexp.QuoteMeta(lea[1])+`\)$`)
}

func TestCodegen_EscapingLocal_CallsRuntimeAllocator(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun escape () -> *int { let a = 10; return &a; }
		fun main () -> int { let p = escape(); return *p; }
	`)

	// the allocator is an ordinary call to a symbol the C runtime defines
	assert.Contains(t, instructions(asm), "call acc_alloc")
}

func TestCodegen_Nil_ZeroesFullWidth(t *testing.T) {
	asm := requireGeneratesAssembly(t, `fun main () -> int { let p *int = nil; let a = 10; p = &a; return *p; }`)

	// a pointer is 8 bytes wide whatever it points at, so nil is zeroed with the quadword move
	assertContainsMatch(t, asm, `^movq \$0, `+quadRegister+`$`)
}

func TestCodegen_Comparison_ComparesAtOperandWidth(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		cmpOp string
		setOp string
	}{
		{name: "int", src: `fun f (a int, b int) -> bool { return a < b; }`, cmpOp: "cmpl", setOp: "setl"},
		{name: "pointer", src: `fun f (p *int) -> bool { return p == nil; }`, cmpOp: "cmpq", setOp: "sete"},
		{name: "bool", src: `fun f (a bool, b bool) -> bool { return a != b; }`, cmpOp: "cmpb", setOp: "setne"},
		{name: "less equal", src: `fun f (a int, b int) -> bool { return a <= b; }`, cmpOp: "cmpl", setOp: "setle"},
		{name: "greater", src: `fun f (a int, b int) -> bool { return a > b; }`, cmpOp: "cmpl", setOp: "setg"},
		{name: "greater equal", src: `fun f (a int, b int) -> bool { return a >= b; }`, cmpOp: "cmpl", setOp: "setge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm := requireGeneratesAssembly(t, tt.src+` fun main () -> int { return 0; }`)

			cmp := requireOnly(t, asm, tt.cmpOp)
			set := requireOnly(t, asm, tt.setOp)
			assertContainsSeq(t, asm, cmp, set)

			// whatever the operands' width, the result is a single byte
			result := operands(set)
			require.Len(t, result, 1)
			assert.Regexp(t, `^`+byteRegister+`$`, result[0])
		})
	}
}

func TestCodegen_Comparison_LeftOperandIsDest(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun f (a int, b int) -> bool { return a < b; }
		fun main () -> int { return 0; }
	`)

	// AT&T cmp computes dest - src, so setl reads a < b only when a is the destination; with nothing else live, each
	// parameter is compared right where it arrived (a in edi, b in esi)
	assert.Contains(t, instructions(asm), "cmpl %esi, %edi")
}

func TestCodegen_BoolLiteral_MovesByteImmediate(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun f () -> bool { return true; }
		fun main () -> int { return 0; }
	`)

	assertContainsMatch(t, asm, `^movb \$1, `+byteRegister+`$`)
}

func TestCodegen_Not_FlipsLowBit(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun f (a bool) -> bool { return !a; }
		fun main () -> int { return 0; }
	`)

	assert.Regexp(t, `^xorb \$1, `+byteRegister+`$`, requireOnly(t, asm, "xorb"))
}

func TestCodegen_Conditional_TestsConditionByte(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun f (c bool) -> int { if c { 1 } else { 2 } }
		fun main () -> int { return 0; }
	`)

	// a bool is a single byte, tested against itself to set the zero flag
	test := requireOnly(t, asm, "testb")
	ops := operands(test)
	require.Len(t, ops, 2)
	assert.Regexp(t, `^`+byteRegister+`$`, ops[0])
	assert.Equal(t, ops[0], ops[1])

	insts := instructions(asm)
	i := slices.Index(insts, test)
	require.Less(t, i+1, len(insts))
	assert.Contains(t, []string{"je", "jne"}, mnemonic(insts[i+1]), "the test feeds a conditional jump")
}

func TestCodegen_Jumps_TargetLabelsDefinedOnce(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun f (n int) -> int { if n < 0 { return 1; }; if n == 0 { 2 } elif n == 1 { 3 } else { 4 } }
		fun g (c bool) -> int { if c { 4 } else { 5 } }
		fun main () -> int { f(0) + g(true) }
	`)

	// block ids restart in every function, so this catches labels that aren't scoped to theirs
	defined := make(map[string]int)
	for _, l := range labels(asm) {
		defined[l] += 1
	}
	for l, n := range defined {
		assert.Equal(t, 1, n, "label %s is defined more than once", l)
	}

	lines := strings.Split(asm, "\n")
	for i, line := range lines {
		inst, ok := strings.CutPrefix(line, "\t")
		if !ok || !slices.Contains([]string{"jmp", "je", "jne"}, mnemonic(inst)) {
			continue
		}
		ops := operands(inst)
		require.Len(t, ops, 1, "%s has no label to jump to", inst)
		assert.Contains(t, defined, ops[0], "%s jumps to an undefined label", inst)
		// the text always ends in a newline, so a jump is never the last line
		assert.NotEqual(t, ops[0]+":", lines[i+1], "%s jumps to the very next instruction", inst)
	}
}

func TestCodegen_ReturnBlocks_ShareOneEpilogue(t *testing.T) {
	asm := requireGeneratesAssembly(t, `
		fun f (c bool) -> int { if c { return 1; } else { return 2; } }
		fun main () -> int { return 0; }
	`)

	// one ret per function, which the return block laid out last falls into and the other jumps to
	assert.Len(t, withMnemonic(asm, "ret"), 2)
	assert.Equal(t, "jmp .Lf.ret", requireOnly(t, asm, "jmp"))
}

func TestX64Inst_String(t *testing.T) {
	tests := []struct {
		name string
		inst X64Inst
		want string
	}{
		{name: "no operands", inst: X64Inst{Op: X64RET}, want: "ret"},
		{name: "dest only", inst: X64Inst{Op: X64NEG, Size: 4, Dst: x64Reg(target.RegA)}, want: "negl %eax"},
		{name: "immediate to register", inst: X64Inst{Op: X64MOV, Size: 8, Src: x64Imm(10), Dst: x64Reg(target.Reg10)}, want: "movq $10, %r10"},
		{name: "numbered register byte", inst: X64Inst{Op: X64MOV, Size: 1, Src: x64Imm(1), Dst: x64Reg(target.Reg9)}, want: "movb $1, %r9b"},
		{name: "numbered register long", inst: X64Inst{Op: X64ADD, Size: 4, Src: x64Reg(target.Reg8), Dst: x64Reg(target.RegSI)}, want: "addl %r8d, %esi"},
		{name: "memory base is full width", inst: X64Inst{Op: X64MOV, Size: 4, Src: x64Reg(target.RegA), Dst: X64Operand{Kind: X64Mem, Reg: target.RegSP, Offset: 16}}, want: "movl %eax, 16(%rsp)"},
		{name: "setcc has no suffix", inst: X64Inst{Op: X64SETL, Size: 1, Dst: x64Reg(target.RegD)}, want: "setl %dl"},
		{name: "sign extend long", inst: X64Inst{Op: X64SignExtendA, Size: 4}, want: "cdq"},
		{name: "sign extend quad", inst: X64Inst{Op: X64SignExtendA, Size: 8}, want: "cqo"},
		{name: "jump to label", inst: X64Inst{Op: X64JMP, Dst: x64Sym(".Lmain.ret")}, want: "jmp .Lmain.ret"},
		{name: "indirect call", inst: X64Inst{Op: X64CALL, Dst: X64Operand{Kind: X64Indirect, Reg: target.Reg11}}, want: "call *%r11"},
		{name: "rip relative", inst: X64Inst{Op: X64LEA, Size: 8, Src: X64Operand{Kind: X64RipRel, Sym: "f"}, Dst: x64Reg(target.RegC)}, want: "leaq f(%rip), %rcx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.inst.String())
		})
	}
}

func TestProgram_String(t *testing.T) {
	p := &Program[X64Inst]{Funcs: []*Func[X64Inst]{{
		Name:   "main",
		Global: true,
		Blocks: []*Block[X64Inst]{
			{Insts: []X64Inst{{Op: X64PUSH, Size: 8, Dst: x64Reg(target.RegBP)}}},
			{Label: ".Lmain.0", Insts: []X64Inst{{Op: X64RET}}},
		},
	}}}

	// an unlabeled block runs straight on from the function's symbol
	assert.Equal(t, "\t.text\n\t.globl main\nmain:\n\tpushq %rbp\n.Lmain.0:\n\tret\n", p.String())
}

// testTarget is the target unit tests generate code for, whatever buildcfg resolved
var testTarget = target.X64SysV

// byteRegister matches any one-byte x64 register
const byteRegister = `%(?:[a-d]l|sil|dil|spl|bpl|r(?:8|9|1[0-5])b)`

// quadRegister matches any eight-byte x64 register
const quadRegister = `%(?:r[a-d]x|rsi|rdi|rsp|rbp|r(?:8|9|1[0-5]))`

// instructions lists the program's instructions without their indentation, leaving out labels and directives.
func instructions(asm string) []string {
	var insts []string
	for _, line := range strings.Split(asm, "\n") {
		inst, ok := strings.CutPrefix(line, "\t")
		if ok && !strings.HasPrefix(inst, ".") {
			insts = append(insts, inst)
		}
	}
	return insts
}

// labels lists the labels the program defines, in layout order.
func labels(asm string) []string {
	var ls []string
	for _, line := range strings.Split(asm, "\n") {
		if l, ok := strings.CutSuffix(line, ":"); ok && !strings.HasPrefix(line, "\t") {
			ls = append(ls, l)
		}
	}
	return ls
}

// mnemonic is the operation an instruction performs.
func mnemonic(inst string) string {
	op, _, _ := strings.Cut(inst, " ")
	return op
}

// operands lists an instruction's operands in AT&T order, sources before the destination.
func operands(inst string) []string {
	_, ops, ok := strings.Cut(inst, " ")
	if !ok {
		return nil
	}
	return strings.Split(ops, ", ")
}

// withMnemonic lists every instruction performing op.
func withMnemonic(asm string, op string) []string {
	var found []string
	for _, inst := range instructions(asm) {
		if mnemonic(inst) == op {
			found = append(found, inst)
		}
	}
	return found
}

// requireOnly returns the sole instruction performing op.
func requireOnly(t *testing.T, asm string, op string) string {
	t.Helper()
	found := withMnemonic(asm, op)
	require.Len(t, found, 1, "expected exactly one %q instruction in\n%s", op, asm)
	return found[0]
}

// indexOfMatch is the position of the first instruction matching pattern, or -1.
func indexOfMatch(asm string, pattern string) int {
	return slices.IndexFunc(instructions(asm), regexp.MustCompile(pattern).MatchString)
}

func assertContainsMatch(t *testing.T, asm string, pattern string) {
	t.Helper()
	assert.NotEqual(t, -1, indexOfMatch(asm, pattern), "no instruction matches %s in\n%s", pattern, asm)
}

// assertContainsSeq checks that the instructions appear in order, though not necessarily back to back.
func assertContainsSeq(t *testing.T, asm string, seq ...string) {
	t.Helper()
	rest := seq
	for _, inst := range instructions(asm) {
		if len(rest) > 0 && inst == rest[0] {
			rest = rest[1:]
		}
	}
	assert.Empty(t, rest, "instructions end before the sequence %q finishes in\n%s", seq, asm)
}

func assertNoSelfMoves(t *testing.T, asm string) {
	t.Helper()
	for _, inst := range instructions(asm) {
		ops := operands(inst)
		if strings.HasPrefix(mnemonic(inst), "mov") && len(ops) == 2 && ops[0] == ops[1] {
			assert.Fail(t, "a mov onto its own location survived codegen", inst)
		}
	}
}

func requireGeneratesAssembly(t *testing.T, src string) string {
	t.Helper()
	return requireGeneratesAssemblyFor(t, buildcfg.OSLinux, src)
}

// requireGeneratesAssemblyFor generates testTarget's assembly for src, naming symbols the way os expects
func requireGeneratesAssemblyFor(t *testing.T, os buildcfg.OS, src string) string {
	t.Helper()
	asm, err := GenerateAssembly(requireBuildSSAFor(t, testTarget, src), WithTarget(testTarget.Arch, os))
	require.NoError(t, err)
	return asm
}

// requireBuildSSAFor builds and allocates src for tgt
func requireBuildSSAFor(t *testing.T, tgt *target.Target, src string) []*ssa.Func {
	t.Helper()

	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	allFuncs, err := semantic.Analyze(funcs)
	require.NoError(t, err)

	ssaFuncs, err := ssa.BuildAndAllocate(allFuncs, ssa.WithTarget(tgt))
	require.NoError(t, err)

	return ssaFuncs
}
