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

	// the call is an instruction alone and produces no value of its own
	assert.Equal(t, LocNone, call.Loc.Kind)
	assert.False(t, call.NeedsRegister())

	// the result is read back out of rax
	f := requireFunc(t, funcs, "main")
	callResults := findValues(f.Entry.Values, OpCallResult)
	require.Len(t, callResults, 1)
	assert.Equal(t, 0, callResults[0].Value)
	assert.Equal(t, LocRegister, callResults[0].Loc.Kind)
	assert.Equal(t, register.Results[0], callResults[0].Loc.Reg)
	require.Len(t, callResults[0].Args, 1)
	assert.Same(t, call, callResults[0].Args[0], "a call result names the call it came from")

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

func TestLowerParams_TupleParamSplitsIntoLeaves(t *testing.T) {
	funcs := requireBuildSSA(t, `fun target (t (int, int, int, ())) -> int { return t.0; }`)

	f := requireFunc(t, funcs, "target")
	params := findValues(f.Entry.Values, OpParam)

	// four items in tuple - 1 unit type that should get ignored
	require.Len(t, params, 3)

	assert.Equal(t, LocRegister, params[0].Loc.Kind)
	assert.Equal(t, register.RegDI, params[0].Loc.Reg)

	assert.Equal(t, LocRegister, params[1].Loc.Kind)
	assert.Equal(t, register.RegSI, params[1].Loc.Reg)

	assert.Equal(t, LocRegister, params[2].Loc.Kind)
	assert.Equal(t, register.RegD, params[2].Loc.Reg)

	// each leaf carries its own atomic type, never the aggregate
	assert.True(t, types.Equal(types.Int(), params[0].Type))
	assert.True(t, types.Equal(types.Int(), params[1].Type))
	assert.True(t, types.Equal(types.Int(), params[2].Type))
}

func TestLowerCalls_TupleArgSplitsIntoLeaves(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (t (int, int, int)) -> int { return t.0; }
		fun main () -> int { let a = (1, 2, 3); return target(a); }
	`)

	call := requireCall(t, funcs, "main")

	// the caller splits the argument the same way the callee expects to receive it
	require.Len(t, call.Args, 3)

	assert.Equal(t, LocRegister, call.Args[0].Loc.Kind)
	assert.Equal(t, register.RegDI, call.Args[0].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[1].Loc.Kind)
	assert.Equal(t, register.RegSI, call.Args[1].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[2].Loc.Kind)
	assert.Equal(t, register.RegD, call.Args[2].Loc.Reg)
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

func TestGenSsa_UnitTupleField_IsNotStored(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun main () -> int {
			let t = (1, (), 41);
			return t.0 + t.2;
		}
	`)

	f := requireFunc(t, funcs, "main")

	assert.Len(t, findValues(f.Entry.Values, OpStaticStore), 2)
	assert.Len(t, findValues(f.Entry.Values, OpStaticLoad), 2)
}

func TestGenSsa_UnitTupleField_IsNotLoaded(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f (t (int, ())) -> int {
			let u = t.1;
			return t.0;
		}
		fun main () -> int { return f((7, ())); }
	`)

	f := requireFunc(t, funcs, "f")

	assert.Len(t, findValues(f.Entry.Values, OpUnit), 1)
	assert.Len(t, findValues(f.Entry.Values, OpStaticLoad), 1)
}

func TestHeapify_RewritesAnEscapingLocal(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () -> *int { let x = 1; return &x; }
		fun main () -> int { let p = f(); return *p; }`)
	f := requireFunc(t, funcs, "f")

	alloc := requireAllocate(t, f)

	// the returned address is what the allocation handed back, copied out of the ABI's return register
	returned := requireReturned(t, f.Entry)
	require.Equal(t, OpCopy, returned.Op)
	require.Len(t, returned.Args, 1)

	result := returned.Args[0]
	require.Equal(t, OpCallResult, result.Op)
	require.Len(t, result.Args, 1)
	assert.Same(t, alloc, result.Args[0])
	assert.True(t, types.Equal(types.Pointer(types.Int()), alloc.Type),
		"expected *int, got %v", alloc.Type)

	// nothing still takes the address of a frame slot, and the slot is gone
	assert.Empty(t, findValues(slices.Collect(f.UnorderedValues()), OpLocalAddr))
	assert.Nil(t, slotNamed(f, "x"))
}

func TestLowerResults_PinnedToResultRegister(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { return 7; }`)

	f := requireFunc(t, funcs, "main")

	// the block hands back exactly one value
	require.Len(t, f.Entry.Control, 1)
	result := f.Entry.Control[0]
	assert.Equal(t, OpResult, result.Op)

	// the result names the register the ABI returns in without occupying one itself
	assert.Equal(t, LocRegister, result.Loc.Kind)
	assert.Equal(t, register.Results[0], result.Loc.Reg)
	assert.False(t, result.NeedsRegister())
	assert.True(t, types.Equal(types.Int(), result.Type))

	// a copy pinned to the same register puts the returned value there
	require.Len(t, result.Args, 1)
	move := result.Args[0]
	assert.Equal(t, OpCopy, move.Op)
	assert.Equal(t, LocRegister, move.Loc.Kind)
	assert.Equal(t, register.Results[0], move.Loc.Reg)

	require.Len(t, move.Args, 1)
	assert.Equal(t, OpLiteral, move.Args[0].Op)
	assert.Equal(t, int32(7), move.Args[0].Value)
}

func TestGenSsa_TupleReturn_SplitsIntoResultRegisters(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun make () -> (int, int) { return (20, 13); }
		fun main () -> int { let r = make(); return r.0 + r.1; }
	`)

	f := requireFunc(t, funcs, "make")

	// the callee hands back one value per leaf, in ABI order
	require.Len(t, f.Entry.Control, 2)

	first := requireResult(t, f.Entry, 0)
	assert.True(t, types.Equal(types.Int(), first.Type))

	second := requireResult(t, f.Entry, 1)
	assert.True(t, types.Equal(types.Int(), second.Type))
}

func TestLowerCallResults_TupleResultSplitsIntoLeaves(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun make () -> (int, int) { return (20, 13); }
		fun main () -> int { let r = make(); return r.0 + r.1; }
	`)

	main := requireFunc(t, funcs, "main")
	call := requireCall(t, funcs, "main")

	// the caller reads the result back out of the same registers the callee handed it over in
	callResults := findValues(main.Entry.Values, OpCallResult)
	require.Len(t, callResults, 2)

	assert.Equal(t, 0, callResults[0].Value)
	assert.Equal(t, LocRegister, callResults[0].Loc.Kind)
	assert.Equal(t, register.Results[0], callResults[0].Loc.Reg)
	require.Len(t, callResults[0].Args, 1)
	assert.Same(t, call, callResults[0].Args[0])

	assert.Equal(t, 1, callResults[1].Value)
	assert.Equal(t, LocRegister, callResults[1].Loc.Kind)
	assert.Equal(t, register.Results[1], callResults[1].Loc.Reg)
	require.Len(t, callResults[1].Args, 1)
	assert.Same(t, call, callResults[1].Args[0])
}

func TestLowerResults_OverflowResultGoesToMemory(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun make () -> (int, int, int) { return (10, 20, 12); }
		fun main () -> int { let r = make(); return r.0 + r.1 + r.2; }
	`)

	f := requireFunc(t, funcs, "make")
	require.Len(t, f.Entry.Control, 3)

	// the first two leaves ride back in the result registers
	requireResult(t, f.Entry, 0)
	requireResult(t, f.Entry, 1)

	// make takes no arguments, so the third leaf goes in the first slot of the caller's region
	third := f.Entry.Control[2]
	assert.Equal(t, OpResult, third.Op)
	assert.Equal(t, LocMemory, third.Loc.Kind)
	assert.Equal(t, register.RegBP, third.Loc.Reg)
	assert.Equal(t, incomingArgOffset(0), third.Loc.Offset)
}

func TestLowerCallResults_OverflowResultReadFromMemory(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun make () -> (int, int, int) { return (10, 20, 12); }
		fun main () -> int { let r = make(); return r.0 + r.1 + r.2; }
	`)

	main := requireFunc(t, funcs, "main")
	callResults := findValues(main.Entry.Values, OpCallResult)
	require.Len(t, callResults, 3)

	assert.Equal(t, LocRegister, callResults[0].Loc.Kind)
	assert.Equal(t, register.Results[0], callResults[0].Loc.Reg)

	assert.Equal(t, LocRegister, callResults[1].Loc.Kind)
	assert.Equal(t, register.Results[1], callResults[1].Loc.Reg)

	// the third comes back out of the bottom of the caller's own frame
	assert.Equal(t, LocMemory, callResults[2].Loc.Kind)
	assert.Equal(t, register.RegSP, callResults[2].Loc.Reg)
	assert.Equal(t, 0, callResults[2].Loc.Offset)

	// which the caller has to reserve room for
	assert.GreaterOrEqual(t, main.maxOutgoingSize(), stackSlotSize)
}

func TestLowerResults_OverflowResultSitsAboveOverflowArguments(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f (a int, b int, c int, d int, e int, g int, h int, i int) -> (int, int, int) {
			return (a + b, c + d, e + g + h + i);
		}
		fun main () -> int { let r = f(1, 2, 3, 4, 5, 6, 7, 8); return r.0 + r.1 + r.2; }
	`)

	// two arguments miss the argument registers, so the overflow result takes the slot above them
	callee := requireFunc(t, funcs, "f")
	require.Len(t, callee.Entry.Control, 3)

	written := callee.Entry.Control[2]
	assert.Equal(t, LocMemory, written.Loc.Kind)
	assert.Equal(t, register.RegBP, written.Loc.Reg)
	assert.Equal(t, incomingArgOffset(2), written.Loc.Offset)

	// the caller names that same slot from its own side of the call
	main := requireFunc(t, funcs, "main")
	callResults := findValues(main.Entry.Values, OpCallResult)
	require.Len(t, callResults, 3)

	read := callResults[2]
	assert.Equal(t, LocMemory, read.Loc.Kind)
	assert.Equal(t, register.RegSP, read.Loc.Reg)
	assert.Equal(t, 2*stackSlotSize, read.Loc.Offset)

	// and reserves the two argument slots plus the result slot above them
	assert.GreaterOrEqual(t, main.maxOutgoingSize(), 3*stackSlotSize)
}

func TestUnoptimized_Nil(t *testing.T) {
	funcs := requireBuildSSA(t, `fun f () -> *int { return nil; } fun main () -> int { let x *int = f(); return 0; }`)

	callee := requireFunc(t, funcs, "f")
	literals := findValues(callee.Entry.Values, OpLiteral)
	require.Len(t, literals, 1)

	// nil is an ordinary zero literal, widened to fill a whole pointer
	assert.Equal(t, int32(0), literals[0].Value)
	assert.True(t, types.Equal(types.Int64(), literals[0].Type))
}

// requireReturned unwraps b's single result and hands back the value feeding it.
func requireReturned(t *testing.T, b *Block) *Value {
	t.Helper()

	require.Len(t, b.Control, 1)
	return requireResult(t, b, 0)
}

// requireResult unwraps the ith value b hands back and returns the value feeding it.
func requireResult(t *testing.T, b *Block, i int) *Value {
	t.Helper()

	require.Greater(t, len(b.Control), i)
	result := b.Control[i]

	require.Equal(t, OpResult, result.Op, "a returning block must control its results")
	require.Equal(t, i, result.Value, "a result carries the index of the register it lands in")
	require.Equal(t, LocRegister, result.Loc.Kind)
	require.Equal(t, register.Results[i], result.Loc.Reg)
	require.Len(t, result.Args, 1)

	// the result is a placeholder, so the copy pinned to its register is what places the value
	move := result.Args[0]
	require.Equal(t, OpCopy, move.Op, "a result must be placed by a copy into its register")
	require.Equal(t, LocRegister, move.Loc.Kind)
	require.Equal(t, register.Results[i], move.Loc.Reg)
	require.Len(t, move.Args, 1)

	return move.Args[0]
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
