package ssa

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenSsa_Basic(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { return 0; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]
	assert.Equal(t, "main", f.Name())

	b := f.Blocks[0]
	assert.Equal(t, BlockRet, b.Kind)

	ret := requireReturned(t, b)
	assert.Equal(t, OpLiteral, ret.Op)
	assert.Equal(t, types.Int(), ret.Type)
}

// TestGenSsa_FoldsToLiteral covers the programs that collapse all the way down to a single constant
func TestGenSsa_FoldsToLiteral(t *testing.T) {
	tests := []struct {
		name string
		fn   string // the function to inspect, "main" when empty
		src  string
		want int32
	}{
		{
			name: "binary operands fold",
			src:  `fun main () -> int { return 1 + 1; }`,
			want: 2,
		},
		{
			name: "addition wraps on overflow",
			src:  fmt.Sprintf(`fun main () -> int { return %d + 1; }`, math.MaxInt32),
			want: math.MinInt32,
		},
		{
			name: "unary operand folds",
			src:  `fun main () -> int { return -10; }`,
			want: -10,
		},
		{
			// both operands promote to constants, so the divide folds away
			name: "promoted variables fold",
			src:  `fun main () -> int { let x = 10; let y = 2; return x / y; }`,
			want: 5,
		},
		{
			name: "assignment overwrites the earlier definition",
			src:  `fun main () -> int { let x = 10; x = 20; return x; }`,
			want: 20,
		},
		{
			name: "compound assignment folds",
			src:  `fun main () -> int { let x = 10; x += 20; return x; }`,
			want: 30,
		},
		{
			name: "reassigned parameter",
			fn:   "f",
			src:  `fun f (x int) -> int { x = 55; return x; }`,
			want: 55,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.fn
			if name == "" {
				name = "main"
			}
			f := requireFunc(t, requireBuildSSA(t, tt.src), name)

			ret := requireReturned(t, f.Entry)
			assert.Equal(t, OpLiteral, ret.Op)
			assert.Equal(t, tt.want, ret.Value)
		})
	}
}

func TestGenSsa_Variable(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; return x; }`)

	b := funcs[0].Blocks[0]

	// mem2reg promotes x, so no memory operations survive
	assert.Empty(t, findValues(b.Values, OpStaticLoad), "load should be promoted away")
	assert.Empty(t, findValues(b.Values, OpStaticStore), "store should be promoted away")

	// nothing names the slot any more, so layout drops it from the frame
	assert.Empty(t, funcs[0].Slots, "promoted slot should be dropped")

	// the stored value flows directly into the return
	ret := requireReturned(t, b)
	assert.Equal(t, OpLiteral, ret.Op)
	assert.Equal(t, int32(10), ret.Value)
}

func TestLowerCalls_ArgRegisters(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3); }
	`)

	call := requireCall(t, funcs, "main")

	// a direct call names its target, so Args holds the arguments alone
	require.Len(t, call.Args, 3)

	callee := call.Callee()
	require.NotNil(t, callee, "call should name its target")
	assert.Equal(t, "target", callee.Name())

	assert.Equal(t, LocRegister, call.Args[0].Loc.Kind)
	assert.Equal(t, register.RegDI, call.Args[0].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[1].Loc.Kind)
	assert.Equal(t, register.RegSI, call.Args[1].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[2].Loc.Kind)
	assert.Equal(t, register.RegD, call.Args[2].Loc.Reg)
}

func TestLowerCalls_ResultAndClobbers(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int) -> int { return 0; }
		fun main () -> int { return target(7); }
	`)

	call := requireCall(t, funcs, "main")

	// the result comes back in rax
	assert.Equal(t, LocRegister, call.Loc.Kind)
	assert.Equal(t, register.RegA, call.Loc.Reg)

	// and the call conservatively clobbers every caller-saved register
	assert.Equal(t, register.CallerSaved, call.Clobbers())
}

func TestGenSsa_Params_PinnedToArgRegisters(t *testing.T) {
	funcs := requireBuildSSA(t, `fun target (a int, b int, c int) -> int { return 0; }`)

	f := requireFunc(t, funcs, "target")
	params := findValues(f.Entry.Values, OpParam)
	require.Len(t, params, 3)

	// each incoming parameter is pinned to its System V argument register, in order
	assert.Equal(t, LocRegister, params[0].Loc.Kind)
	assert.Equal(t, register.RegDI, params[0].Loc.Reg)

	assert.Equal(t, LocRegister, params[1].Loc.Kind)
	assert.Equal(t, register.RegSI, params[1].Loc.Reg)

	assert.Equal(t, LocRegister, params[2].Loc.Kind)
	assert.Equal(t, register.RegD, params[2].Loc.Reg)
}

func TestGenSsa_Param_FlowsToReturn(t *testing.T) {
	// returning a parameter used to fail with "variable used before declared"
	funcs := requireBuildSSA(t, `fun identity (x int) -> int { return x; }`)

	f := requireFunc(t, funcs, "identity")

	// the parameter is copied out of its argument register, then into the return register
	ret := requireReturned(t, f.Entry)
	require.Equal(t, OpCopy, ret.Op)

	require.Len(t, ret.Args, 1)
	assert.Equal(t, OpParam, ret.Args[0].Op)
}

func TestLowerCalls_StackArgs(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int, d int, e int, f int, g int, h int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3, 4, 5, 6, 7, 8); }
	`)

	call := requireCall(t, funcs, "main")

	require.Len(t, call.Args, 8)

	// the first six still go in the System V argument registers
	assert.Equal(t, LocRegister, call.Args[0].Loc.Kind)
	assert.Equal(t, register.RegDI, call.Args[0].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[5].Loc.Kind)
	assert.Equal(t, register.Reg9, call.Args[5].Loc.Reg)

	// the seventh and eighth are written into the outgoing area, lowest slot first
	assert.Equal(t, LocMemory, call.Args[6].Loc.Kind)
	assert.Equal(t, register.RegSP, call.Args[6].Loc.Reg)

	assert.Equal(t, LocMemory, call.Args[7].Loc.Kind)
	assert.Equal(t, register.RegSP, call.Args[7].Loc.Reg)

	assert.Less(t, call.Args[6].Loc.Offset, call.Args[7].Loc.Offset)
}

func TestLowerParams_StackParams(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int, d int, e int, f int, g int, h int) -> int { return 0; }
	`)

	f := requireFunc(t, funcs, "target")
	params := findValues(f.Entry.Values, OpParam)
	require.Len(t, params, 8)

	// the sixth parameter is the last one to arrive in a register
	assert.Equal(t, LocRegister, params[5].Loc.Kind)
	assert.Equal(t, register.Reg9, params[5].Loc.Reg)

	// the rest arrive in the caller's frame, past the saved rbp and the return address
	assert.Equal(t, LocMemory, params[6].Loc.Kind)
	assert.Equal(t, register.RegBP, params[6].Loc.Reg)

	assert.Equal(t, LocMemory, params[7].Loc.Kind)
	assert.Equal(t, register.RegBP, params[7].Loc.Reg)

	// above rbp is the caller's frame; below it would collide with our own locals
	assert.Positive(t, params[6].Loc.Offset)
	assert.Less(t, params[6].Loc.Offset, params[7].Loc.Offset)
}

func TestLayoutFrame_ReservesOutgoingArea(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int, d int, e int, f int, g int, h int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3, 4, 5, 6, 7, 8); }
	`)

	main := requireFunc(t, funcs, "main")

	// two arguments spill to the stack, so the adjustment folds in their two eightbyte slots
	assert.GreaterOrEqual(t, main.StackAdjustment(), 16)

	// the whole frame stays 16-byte aligned so rsp is aligned at the call
	pushBytes := (main.UsedRegisters() & register.CalleeSaved).Count() * 8
	assert.Equal(t, 0, (pushBytes+main.StackAdjustment())%16)

	// target makes no calls and has no locals, so it subtracts nothing
	assert.Equal(t, 0, requireFunc(t, funcs, "target").StackAdjustment())
}

func TestMaxOutgoingSize_WidestCallWins(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun small (a int, b int, c int, d int, e int, f int, g int) -> int { return 0; }
		fun big (a int, b int, c int, d int, e int, f int, g int, h int, i int) -> int { return 0; }
		fun main () -> int { return small(1, 2, 3, 4, 5, 6, 7) + big(1, 2, 3, 4, 5, 6, 7, 8, 9); }
	`)

	// the outgoing area is reused across calls, so it fits the widest
	assert.Equal(t, 24, requireFunc(t, funcs, "main").maxOutgoingSize())
}

func TestGenSsa_Ref_KeepsAddressedSlotInFrame(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	f := requireFunc(t, funcs, "main")

	// taking a's address blocks promotion, but b is only ever loaded so it still promotes
	require.Len(t, f.Slots, 1)
	slot := f.Slots[0]
	require.NotNil(t, slot.Sym)
	assert.Equal(t, "a", slot.Sym.Name)

	// the surviving slot gets a real home below rbp, not a register
	assert.Equal(t, LocMemory, slot.Loc.Kind)
	assert.Equal(t, register.RegBP, slot.Loc.Reg)
}

func TestGenSsa_Ref_MaterializesSlotAddress(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	f := requireFunc(t, funcs, "main")
	addrs := findValues(f.Entry.Values, OpLocalAddr)
	require.Len(t, addrs, 1)

	// the address names the slot it points at and lands in a register
	require.Len(t, f.Slots, 1)
	assert.Same(t, f.Slots[0], addrs[0].Slot())
	assert.True(t, types.Equal(types.Pointer(types.Int()), addrs[0].Type))
	assert.Equal(t, LocRegister, addrs[0].Loc.Kind)
	assert.Empty(t, addrs[0].Args, "a slot address takes no operand")
}

func TestGenSsa_Deref_LoadsThroughPointer(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let a = 10; let b = &a; return *b; }`)

	f := requireFunc(t, funcs, "main")

	ret := requireReturned(t, f.Entry)
	require.Equal(t, OpLoad, ret.Op)

	// an indirect load carries its address as an operand and names no slot
	assert.Nil(t, ret.Slot())
	require.Len(t, ret.Args, 1)
	assert.Equal(t, OpLocalAddr, ret.Args[0].Op)
	assert.True(t, types.Equal(types.Int(), ret.Type))
}

func TestGenSsa_Deref_AssignmentStoresThroughPointer(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let a = 10; let b = &a; *b = 20; return a; }`)

	f := requireFunc(t, funcs, "main")
	stores := findValues(f.Entry.Values, OpStore)
	require.Len(t, stores, 1)
	store := stores[0]

	assert.Nil(t, store.Slot())
	require.Len(t, store.Args, 2)
	assert.Equal(t, OpLiteral, store.Args[0].Op)
	assert.Equal(t, int32(20), store.Args[0].Value)
	assert.Equal(t, OpLocalAddr, store.Args[1].Op)

	// a store produces nothing, so it never claims a register
	assert.False(t, store.NeedsRegister())
	assert.Equal(t, LocNone, store.Loc.Kind)
}

func TestGenSsa_DerefAssignOp_EvaluatesAddressOnce(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let a = 10; let b = &a; *b += 5; return a; }`)

	f := requireFunc(t, funcs, "main")

	// *b += 5 computes the destination address once and both reads and writes through it
	addrs := findValues(f.Entry.Values, OpLocalAddr)
	require.Len(t, addrs, 1)

	loads := findValues(f.Entry.Values, OpLoad)
	require.Len(t, loads, 1)
	require.Len(t, loads[0].Args, 1)
	assert.Same(t, addrs[0], loads[0].Args[0])

	stores := findValues(f.Entry.Values, OpStore)
	require.Len(t, stores, 1)
	require.Len(t, stores[0].Args, 2)
	assert.Same(t, addrs[0], stores[0].Args[1])
}

func TestGenSsa_RefOfDeref_ReusesPointer(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let a = 10; let b = &a; let c = &*b; return *c; }`)

	f := requireFunc(t, funcs, "main")

	// &*b is just b, so a's address is the only one ever computed
	assert.Len(t, findValues(f.Entry.Values, OpLocalAddr), 1)
	assert.Len(t, f.Slots, 1)
}

func TestGenSsa_UnitReturn_HasNoControlValue(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () { return; }
		fun main () -> int { return 0; }
	`)

	f := requireFunc(t, funcs, "f")

	// a bare return still terminates the block, but there is no value to place in the return register
	assert.Equal(t, BlockRet, f.Entry.Kind)
	assert.Nil(t, f.Entry.Control)
	assert.Empty(t, f.Entry.Values)
}

func TestGenSsa_UnitFunction_ImplicitReturn(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () { }
		fun main () -> int { return 0; }
	`)

	f := requireFunc(t, funcs, "f")

	// falling off the end terminates the block just like an explicit bare return does
	assert.Equal(t, BlockRet, f.Entry.Kind)
	assert.Nil(t, f.Entry.Control)
}

func TestGenSsa_UnitFunction_CallStatement(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () { return; }
		fun g () { f(); return; }
		fun main () -> int { return 0; }
	`)

	g := requireFunc(t, funcs, "g")
	assert.Equal(t, BlockRet, g.Entry.Kind)
	assert.Nil(t, g.Entry.Control)

	call := requireCall(t, funcs, "g")
	require.NotNil(t, call.Callee())
	assert.Equal(t, "f", call.Callee().Name())
	assert.Empty(t, call.Args)
	assert.True(t, types.Equal(types.Unit(), call.Type))
}

func TestHeapify_RewritesAnEscapingLocal(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () -> *int { let x = 1; return &x; }
		fun main () -> int { let p = f(); return *p; }`)
	f := requireFunc(t, funcs, "f")

	alloc := requireAllocate(t, f)

	// the returned address is the allocation, copied out of the ABI's return register
	returned := requireReturned(t, f.Entry)
	require.Equal(t, OpCopy, returned.Op)
	require.Len(t, returned.Args, 1)
	assert.Equal(t, alloc, returned.Args[0])
	assert.True(t, types.Equal(types.Pointer(types.Int()), alloc.Type),
		"expected *int, got %v", alloc.Type)

	// nothing still takes the address of a frame slot, and the slot is gone
	assert.Empty(t, findValues(slices.Collect(f.UnorderedValues()), OpLocalAddr))
	assert.Nil(t, slotNamed(f, "x"))
}

// requireReturned unwraps b's return copy and hands back the value feeding it.
func requireReturned(t *testing.T, b *Block) *Value {
	t.Helper()

	require.NotNil(t, b.Control)
	require.Equal(t, OpCopy, b.Control.Op, "a returning block must end in the return copy")
	require.Equal(t, LocRegister, b.Control.Loc.Kind)
	require.Equal(t, register.ReturnTarget, b.Control.Loc.Reg)
	require.Len(t, b.Control.Args, 1)

	return b.Control.Args[0]
}

func requireBuildSSA(t *testing.T, src string) []*Func {
	t.Helper()
	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)
	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)
	newFuncs, err := semantic.Analyze(funcs)
	require.NoError(t, err)
	result, err := BuildAndAllocate(newFuncs)
	require.NoError(t, err)
	return result
}

func findValues(values []*Value, op Op) []*Value {
	var result []*Value
	for _, v := range values {
		if v.Op == op {
			result = append(result, v)
		}
	}
	return result
}

func requireFunc(t *testing.T, funcs []*Func, name string) *Func {
	t.Helper()
	for _, f := range funcs {
		if f.Name() == name {
			return f
		}
	}
	require.Failf(t, "function not found", "no function named %q", name)
	return nil
}

// requireCall returns the single OpCall value in the named function.
func requireCall(t *testing.T, funcs []*Func, funcName string) *Value {
	t.Helper()
	f := requireFunc(t, funcs, funcName)
	calls := findValues(f.Entry.Values, OpStaticCall)
	require.Len(t, calls, 1)
	return calls[0]
}

// requireAllocate returns the single call to the runtime allocator in f.
func requireAllocate(t *testing.T, f *Func) *Value {
	t.Helper()
	allocs := findAllocations(f)
	require.Len(t, allocs, 1)
	return allocs[0]
}

// findAllocations returns every call f makes to the runtime allocator.
func findAllocations(f *Func) []*Value {
	var result []*Value
	for v := range f.UnorderedValues() {
		if v.Op == OpStaticCall && v.Callee() == Alloc {
			result = append(result, v)
		}
	}
	return result
}

// slotNamed returns the slot named sym, or nil when f no longer has one.
func slotNamed(f *Func, sym string) *Slot {
	for _, s := range f.Slots {
		if s.Sym.Name == sym {
			return s
		}
	}
	return nil
}
