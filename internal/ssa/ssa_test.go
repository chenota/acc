package ssa

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/target"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenSsa_FoldsToLiteral covers the programs that collapse all the way down to a single constant
func TestGenSsa_FoldsToLiteral(t *testing.T) {
	tests := []struct {
		name string
		fn   string // the function to inspect, "main" when empty
		src  string
		want int32
	}{
		{
			name: "literal",
			src:  `fun main () -> int { return 0; }`,
			want: 0,
		},
		{
			name: "variable",
			src:  `fun main () -> int { let x = 10; return x; }`,
			want: 10,
		},
		{
			name: "tuple field",
			src:  `fun main () -> int { let t = (1, 2); return t.1; }`,
			want: 2,
		},
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
		{
			name: "promoted tuple fields fold",
			src:  `fun main () -> int { let t = (20, 22); return t.0 + t.1; }`,
			want: 42,
		},
		{
			name: "tuple fields are promoted independently",
			src:  `fun main () -> int { let t = (1, 2); t.0 = 5; return t.1; }`,
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.fn
			if name == "" {
				name = "main"
			}
			f := requireFunc(t, requireBuildSSA(t, tt.src), name)

			assert.Equal(t, BlockRet, f.Entry.Kind)

			ret := requireReturned(t, f.Entry)
			assert.Equal(t, OpLiteral, ret.Op)
			assert.Equal(t, types.Int(), ret.Type)
			assert.Equal(t, tt.want, ret.Value)

			// mem2reg promotes everything, so no memory operations survive and layout has no slot left to keep
			assert.Empty(t, findValues(f.Entry.Values, OpStaticLoad), "loads should be promoted away")
			assert.Empty(t, findValues(f.Entry.Values, OpStaticStore), "stores should be promoted away")
			assert.Empty(t, f.Slots, "promoted slots should be dropped")
		})
	}
}

// TestGenSsa_BoolFoldsToLiteral covers comparisons and nots that collapse to a constant bool
func TestGenSsa_BoolFoldsToLiteral(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "literal",
			src:  `fun f () -> bool { return true; }`,
			want: true,
		},
		{
			name: "less than",
			src:  `fun f () -> bool { return 1 < 2; }`,
			want: true,
		},
		{
			name: "less equal on equal operands",
			src:  `fun f () -> bool { return 2 <= 2; }`,
			want: true,
		},
		{
			name: "greater than",
			src:  `fun f () -> bool { return 1 > 2; }`,
			want: false,
		},
		{
			name: "greater equal",
			src:  `fun f () -> bool { return 1 >= 2; }`,
			want: false,
		},
		{
			name: "signed ordering",
			src:  `fun f () -> bool { return -1 < 0; }`,
			want: true,
		},
		{
			name: "int equality",
			src:  `fun f () -> bool { return 4 == 4; }`,
			want: true,
		},
		{
			name: "int inequality",
			src:  `fun f () -> bool { return 4 != 4; }`,
			want: false,
		},
		{
			name: "bool equality",
			src:  `fun f () -> bool { return true == false; }`,
			want: false,
		},
		{
			name: "bool inequality",
			src:  `fun f () -> bool { return true != false; }`,
			want: true,
		},
		{
			name: "not",
			src:  `fun f () -> bool { return !true; }`,
			want: false,
		},
		{
			// the add folds first, and the comparison then folds on the result
			name: "operands fold before comparing",
			src:  `fun f () -> bool { return (1 + 2) < 4; }`,
			want: true,
		},
		{
			name: "promoted variables fold",
			src:  `fun f () -> bool { let x = 3; let y = 3; return x == y; }`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, tt.src), "f")

			ret := requireReturned(t, f.Entry)
			assert.Equal(t, OpLiteral, ret.Op)
			assert.Equal(t, types.Bool(), ret.Type)
			assert.Equal(t, tt.want, ret.Value)
		})
	}
}

func TestGenSsa_Comparison_ProducesBoolFromOperands(t *testing.T) {
	tests := []struct {
		name string
		src  string
		op   Op
	}{
		{name: "equal", src: `fun f (a int, b int) -> bool { return a == b; }`, op: OpEqual},
		{name: "not equal", src: `fun f (a int, b int) -> bool { return a != b; }`, op: OpNotEqual},
		{name: "less", src: `fun f (a int, b int) -> bool { return a < b; }`, op: OpLess},
		{name: "less equal", src: `fun f (a int, b int) -> bool { return a <= b; }`, op: OpLessEqual},
		{name: "greater", src: `fun f (a int, b int) -> bool { return a > b; }`, op: OpGreater},
		{name: "greater equal", src: `fun f (a int, b int) -> bool { return a >= b; }`, op: OpGreaterEqual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, tt.src), "f")

			cmp := requireReturned(t, f.Entry)
			assert.Equal(t, tt.op, cmp.Op)
			assert.Equal(t, types.Bool(), cmp.Type)

			// operands keep their order, since every comparison but equality is asymmetric
			require.Len(t, cmp.Args, 2)
			assert.Equal(t, 0, throughCopies(cmp.Args[0]).Value, "left operand is the first parameter")
			assert.Equal(t, 1, throughCopies(cmp.Args[1]).Value, "right operand is the second parameter")
		})
	}
}

func TestGenSsa_Not_KeepsOperand(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `fun f (a bool) -> bool { return !a; }`), "f")

	not := requireReturned(t, f.Entry)
	assert.Equal(t, OpNot, not.Op)
	assert.Equal(t, types.Bool(), not.Type)
	require.Len(t, not.Args, 1)
	assert.Equal(t, OpParam, throughCopies(not.Args[0]).Op)
}

func TestUnaryFold_BranchConditionIsKept(t *testing.T) {
	// debug is read by the not, which folds away, and by the branch, which doesn't
	f := requireFunc(t, requireBuildSSA(t, `
		fun f () -> int { let debug = false; let quiet = !debug; if debug { 1 } else { 2 } }`), "f")
	assert.Empty(t, findValues(f.Entry.Values, OpNot), "the not folds away")

	require.Equal(t, BlockIf, f.Entry.Kind)
	require.Len(t, f.Entry.Control, 1, "the branch keeps its condition")
	assert.Equal(t, OpLiteral, f.Entry.Control[0].Op)
	assert.Equal(t, false, f.Entry.Control[0].Value)
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
	assert.Equal(t, testTarget.Results[0], callResults[0].Loc.Reg)
	require.Len(t, callResults[0].Args, 1)
	assert.Same(t, call, callResults[0].Args[0], "a call result names the call it came from")

	// and the call conservatively clobbers every caller-saved register
	assert.Equal(t, testTarget.CallerSaved, call.regs().clobbers)
}

func TestLowerCalls_StackArgs(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int, d int, e int, f int, g int, h int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3, 4, 5, 6, 7, 8); }
	`)

	call := requireCall(t, funcs, "main")

	require.Len(t, call.Args, 8)

	// the first six still go in the System V argument registers
	assertInArgRegisters(t, call.Args[:6])

	// the seventh and eighth are written into the outgoing area, lowest slot first
	assert.Equal(t, LocMemory, call.Args[6].Loc.Kind)
	assert.Equal(t, target.RegSP, call.Args[6].Loc.Reg)

	assert.Equal(t, LocMemory, call.Args[7].Loc.Kind)
	assert.Equal(t, target.RegSP, call.Args[7].Loc.Reg)

	assert.Less(t, call.Args[6].Loc.Offset, call.Args[7].Loc.Offset)
}

func TestLowerParams_StackParams(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int, d int, e int, f int, g int, h int) -> int { return 0; }
	`)

	f := requireFunc(t, funcs, "target")
	params := findValues(f.Entry.Values, OpParam)
	require.Len(t, params, 8)

	// the first six are pinned to their System V argument registers, in order
	assertInArgRegisters(t, params[:6])

	// the rest arrive in the caller's frame, past the saved rbp and the return address
	assert.Equal(t, LocMemory, params[6].Loc.Kind)
	assert.Equal(t, target.RegBP, params[6].Loc.Reg)

	assert.Equal(t, LocMemory, params[7].Loc.Kind)
	assert.Equal(t, target.RegBP, params[7].Loc.Reg)

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
	pushBytes := main.SavedRegisters().Count() * 8
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
	assertInArgRegisters(t, params)

	// each leaf carries its own atomic type, never the aggregate
	for _, p := range params {
		assert.True(t, types.Equal(types.Int(), p.Type))
	}
}

func TestLowerCalls_TupleArgSplitsIntoLeaves(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (t (int, int, int)) -> int { return t.0; }
		fun main () -> int { let a = (1, 2, 3); return target(a); }
	`)

	call := requireCall(t, funcs, "main")

	// the caller splits the argument the same way the callee expects to receive it
	require.Len(t, call.Args, 3)
	assertInArgRegisters(t, call.Args)
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
	assert.Equal(t, target.RegBP, slot.Loc.Reg)

	// the address names the slot it points at and lands in a register
	addrs := findValues(f.Entry.Values, OpLocalAddr)
	require.Len(t, addrs, 1)
	assert.Same(t, slot, addrs[0].Slot())
	assert.True(t, types.Equal(types.Pointer(types.Int()), addrs[0].Type))
	assert.Equal(t, LocRegister, addrs[0].Loc.Kind)
	assert.Empty(t, addrs[0].Args, "a slot address takes no operand")

	// an indirect load carries its address as an operand and names no slot
	ret := requireReturned(t, f.Entry)
	require.Equal(t, OpLoad, ret.Op)
	assert.Nil(t, ret.Slot())
	require.Len(t, ret.Args, 1)
	assert.Same(t, addrs[0], ret.Args[0])
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
	tests := []struct {
		name string
		src  string
	}{
		{name: "bare return", src: `fun f () { return; }`},
		{name: "falling off the end", src: `fun f () { }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, tt.src), "f")

			// the block still terminates, but there is no value to place in the return register
			assert.Equal(t, BlockRet, f.Entry.Kind)
			assert.Nil(t, f.Entry.Control)
			assert.Empty(t, f.Entry.Values)
		})
	}
}

func TestGenSsa_UnitFunction_CallStatement(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () { return; }
		fun g () { f(); return; }`)

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
	// taking t's address keeps it in memory so its stores survive mem2reg
	funcs := requireBuildSSA(t, `
		fun main () -> int {
			let t = (1, (), 41);
			let p = &t;
			return t.0 + t.2;
		}
	`)

	f := requireFunc(t, funcs, "main")

	assert.Len(t, findValues(f.Entry.Values, OpStaticStore), 2)
	assert.Len(t, findValues(f.Entry.Values, OpStaticLoad), 2)
}

func TestGenSsa_UnitTupleField_IsNotLoaded(t *testing.T) {
	// taking t's address keeps it in memory so its loads survive mem2reg
	funcs := requireBuildSSA(t, `
		fun f (t (int, ())) -> int {
			let p = &t;
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

// TestHeapify_NothingIsLeftOnTheFrame covers functions where everything the closure reaches outlives the frame
func TestHeapify_NothingIsLeftOnTheFrame(t *testing.T) {
	tests := []struct {
		name   string
		fn     string
		src    string
		allocs int
	}{
		{
			// the closure carries its own copy of x along, so only its environment moves
			name:   "escaping environment",
			fn:     "adderFactory",
			allocs: 1,
			src: `
				fun adderFactory (x int) -> fun (int) -> int { return fun (y int) -> int { return x + y; }; }
				fun main () -> int { let f = adderFactory(1); return 2; }`,
		},
		{
			// the environment and g's slot point at each other, and both outlive the frame
			name:   "escaping let rec",
			fn:     "mk",
			allocs: 2,
			src: `
				fun mk () -> fun () -> int { let rec g = fun () -> int { let h = g; return 5; }; return g; }
				fun main () -> int { let f = mk(); return 2; }`,
		},
		{
			// f holds a closure that holds r, which points back at f; f and the environment outlive the frame,
			// while r is never written so the closure keeps a copy and r needs no storage of its own
			// the first lambda captures nothing, so it has no environment to allocate
			name:   "address cycle",
			fn:     "mk",
			allocs: 2,
			src: `
				fun mk () -> fun () -> int {
					let f = fun () -> int { return 1; };
					let r = &f;
					f = fun () -> int { let q = r; return 7; };
					return f;
				}
				fun main () -> int { let g = mk(); return 2; }`,
		},
		{
			// g reaches its own environment through its box, so calling it leaks both the environment and g's box
			name:   "closure leaking itself through a capture",
			fn:     "mk",
			allocs: 2,
			src: `
				fun mk (r *fun () -> int) {
					let rec g = fun () -> int { *r = g; return 1; };
					g();
				}
				fun main () -> int { let h = fun () -> int { return 0; }; mk(&h); return h(); }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, tt.src), tt.fn)

			assert.Len(t, findAllocations(f), tt.allocs)
			assert.Empty(t, f.Slots, "nothing is left on the frame to point at")
			assert.Empty(t, findValues(slices.Collect(f.UnorderedValues()), OpLocalAddr))
		})
	}
}

func TestHeapify_CopyIntoEscapingSlotStaysOnFrame(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () -> *int { let z = 1; let pz = &z; let l = *pz; return &l; }
		fun main () -> int { let p = f(); return *p; }`)
	f := requireFunc(t, funcs, "f")

	// l escapes, but it only holds a copy of z's value, so z keeps its frame slot
	requireAllocate(t, f)
	assert.Nil(t, slotNamed(f, "l"))
	assert.NotNil(t, slotNamed(f, "z"))
}

func TestEscapeSummary_SelfRecursionAssumesTheWorst(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f (p *int) -> *int { return f(p); }`)
	f := requireFunc(t, funcs, "f")

	// the call back into f can't use a summary that's still being computed, so p is handed to it as if it escapes
	require.Len(t, f.escapes, 1)
	require.NotNil(t, f.escapes[0].heap)
	assert.Equal(t, 0, *f.escapes[0].heap)
}

func TestEscapeSummary_MutualRecursionTerminates(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun even (p *int) -> *int { return odd(p); }
		fun odd (p *int) -> *int { return even(p); }`)

	for _, name := range []string{"even", "odd"} {
		f := requireFunc(t, funcs, name)
		require.Len(t, f.escapes, 1, name)
		assert.NotNil(t, f.escapes[0].heap, name)
	}
}

func TestLowerResults_OverflowResultGoesToMemory(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun make () -> (int, int, int) { return (10, 20, 12); }
		fun main () -> int { let r = make(); return r.0 + r.1 + r.2; }
	`)

	f := requireFunc(t, funcs, "make")
	require.Len(t, f.Entry.Control, 3)

	// the first two leaves ride back in the result registers, in ABI order
	assert.True(t, types.Equal(types.Int(), requireResult(t, f.Entry, 0).Type))
	assert.True(t, types.Equal(types.Int(), requireResult(t, f.Entry, 1).Type))

	// make takes no arguments, so the third leaf goes in the first slot of the caller's region
	third := f.Entry.Control[2]
	assert.Equal(t, OpResult, third.Op)
	assert.Equal(t, LocMemory, third.Loc.Kind)
	assert.Equal(t, target.RegBP, third.Loc.Reg)
	assert.Equal(t, incomingArgOffset(0), third.Loc.Offset)
}

func TestLowerCallResults_OverflowResultReadFromMemory(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun make () -> (int, int, int) { return (10, 20, 12); }
		fun main () -> int { let r = make(); return r.0 + r.1 + r.2; }
	`)

	main := requireFunc(t, funcs, "main")
	call := requireCall(t, funcs, "main")
	callResults := findValues(main.Entry.Values, OpCallResult)
	require.Len(t, callResults, 3)

	for i, result := range callResults {
		assert.Equal(t, i, result.Value)
		require.Len(t, result.Args, 1)
		assert.Same(t, call, result.Args[0])
	}

	// the caller reads the first two back out of the same registers the callee handed them over in
	assert.Equal(t, NewReg(testTarget.Results[0]), callResults[0].Loc)
	assert.Equal(t, NewReg(testTarget.Results[1]), callResults[1].Loc)

	// the third comes back out of the bottom of the caller's own frame
	assert.Equal(t, NewMem(target.RegSP, 0), callResults[2].Loc)

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
	assert.Equal(t, target.RegBP, written.Loc.Reg)
	assert.Equal(t, incomingArgOffset(2), written.Loc.Offset)

	// the caller names that same slot from its own side of the call
	main := requireFunc(t, funcs, "main")
	callResults := findValues(main.Entry.Values, OpCallResult)
	require.Len(t, callResults, 3)

	read := callResults[2]
	assert.Equal(t, LocMemory, read.Loc.Kind)
	assert.Equal(t, target.RegSP, read.Loc.Reg)
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

func TestHeapify_CalledClosureEnvironmentStaysOnFrame(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun mk () -> *int {
			let x = 1;
			let f = fun () -> *int { return &x; };
			return f();
		}
		fun main () -> int { let p = mk(); return *p; }`)

	f := requireFunc(t, funcs, "mk")

	// calling a closure hands the callee its environment, but the callee has no way to leak the environment itself
	requireEnvSlot(t, f)

	// what the environment points at is still fair game, and this callee hands x's box back out
	requireAllocate(t, f)
	assert.Nil(t, slotNamed(f, "x"))
}

func TestHeapify_VariableDeclaredInLoopIsAllocatedInLoop(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f (n int) -> *int {
			let i = 0;
			loop {
				let j = i;
				if j == n { return &j; };
				i = i + 1;
			}
		}
		fun main () -> int { return *f(1); }`)
	f := requireFunc(t, funcs, "f")

	alloc := requireAllocate(t, f)
	assert.NotNil(t, f.Loops()[alloc.Block])
}

func TestHeapify_VariableDeclaredBeforeLoopIsAllocatedOutsideIt(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f (n int) -> *int {
			let x = 0;
			let i = 0;
			loop {
				if i == n { break; };
				x = x + 1;
				i = i + 1;
			};
			&x
		}
		fun main () -> int { return *f(1); }`)
	f := requireFunc(t, funcs, "f")

	alloc := requireAllocate(t, f)
	assert.Nil(t, f.Loops()[alloc.Block])
}

func TestHeapify_AddressCarriedAroundLoopIsHeapified(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun main () -> int {
			let i = 0;
			let z = 100;
			let first = &z;
			loop {
				if i == 3 { break; };
				let j = i;
				if i == 0 { first = &j; };
				i = i + 1;
			};
			*first
		}`), "main")

	// the next iteration's j can't share storage with the one first points at
	alloc := requireAllocate(t, f)
	assert.NotNil(t, f.Loops()[alloc.Block])
	assert.Nil(t, slotNamed(f, "j"))

	// there's only ever one z, however many times the loop carries its address
	assert.NotNil(t, slotNamed(f, "z"))
}

func TestHeapify_AddressHeldBySlotOutsideLoopIsHeapified(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun main () -> int {
			let i = 0;
			let z = 100;
			let first = &z;
			let holder = &first;
			loop {
				if i == 3 { break; };
				let j = i;
				if i == 0 { first = &j; };
				i = i + 1;
			};
			let p = *holder;
			*p
		}`), "main")

	// first stays in memory, so no phi carries the address
	require.NotNil(t, slotNamed(f, "first"))

	alloc := requireAllocate(t, f)
	assert.NotNil(t, f.Loops()[alloc.Block])
	assert.Nil(t, slotNamed(f, "j"))
}

func TestHeapify_ClosureCarriedAroundLoopHasItsEnvironmentHeapified(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun main () -> int {
			let i = 0;
			let first = fun () -> int { 0 };
			loop {
				if i == 3 { break; };
				let j = i;
				let g = fun () -> int { j };
				if i == 0 { first = g; };
				i = i + 1;
			};
			first()
		}`), "main")

	alloc := requireAllocate(t, f)
	assert.NotNil(t, f.Loops()[alloc.Block])
	assert.Empty(t, f.Slots)
}

func TestHeapify_ClosureUsedWithinIterationStaysOnFrame(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun main () -> int {
			let i = 0;
			let s = 0;
			loop {
				if i == 3 { break; };
				let j = i;
				let g = fun () -> int { j };
				s = s + g();
				i = i + 1;
			};
			s
		}`), "main")

	requireEnvSlot(t, f)
	assert.Empty(t, findAllocations(f))
}

func TestHeapify_AddressCarriedOnlyAroundInnerLoopStaysOnFrame(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun main () -> int {
			let i = 0;
			let s = 0;
			loop {
				if i == 2 { break; };
				let j = i;
				let y = 1;
				let p = &j;
				let k = 0;
				loop {
					if k == 2 { break; };
					s = s + *p;
					p = &y;
					k = k + 1;
				};
				i = i + 1;
			};
			s
		}`), "main")

	// the inner loop's iterations all see the same j, and p is gone by the time the outer loop goes around
	assert.Empty(t, findAllocations(f))
	assert.NotNil(t, slotNamed(f, "j"))
	assert.NotNil(t, slotNamed(f, "y"))
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
	require.Equal(t, testTarget.Results[i], result.Loc.Reg)
	require.False(t, result.NeedsRegister(), "a result names its register without occupying one")
	require.Len(t, result.Args, 1)

	// the result is a placeholder, so the copy pinned to its register is what places the value
	move := result.Args[0]
	require.Equal(t, OpCopy, move.Op, "a result must be placed by a copy into its register")
	require.Equal(t, LocRegister, move.Loc.Kind)
	require.Equal(t, testTarget.Results[i], move.Loc.Reg)
	require.Len(t, move.Args, 1)
	require.True(t, types.Equal(move.Args[0].Type, result.Type), "a result carries the type of what it returns")

	return move.Args[0]
}

func requireBuildSSA(t *testing.T, src string) []*Func {
	t.Helper()
	return requireBuildSSAFor(t, testTarget, src)
}

// requireBuildSSAFor builds and allocates src for tgt
func requireBuildSSAFor(t *testing.T, tgt *target.Target, src string) []*Func {
	t.Helper()
	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)
	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)
	newFuncs, err := semantic.Analyze(funcs)
	require.NoError(t, err)
	result, err := BuildAndAllocate(newFuncs, WithTarget(tgt))
	require.NoError(t, err)
	return result
}

// testTarget is the target unit tests compile for, whatever buildcfg resolved
var testTarget = target.X64SysV

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
		if s.Sym != nil && s.Sym.Name == sym {
			return s
		}
	}
	return nil
}

func TestGenSsa_Closure_ImmutableCaptureIsCopied(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; let f = fun (y int) -> int { return x + y; }; return x; }`)

	f := requireFunc(t, funcs, "main")
	env := requireEnvSlot(t, f)

	// the code pointer travels in the closure itself, so the environment is just the captured variables
	assert.True(t, types.Equal(types.Tuple([]*types.Type{types.Int()}), env.Type),
		"expected (int,), got %v", env.Type)

	// nothing ever writes x, so the only field holds a copy of its value and x needs no storage of its own
	capture := requireStoredAt(t, f, env, 0)
	require.Equal(t, OpLiteral, capture.Op)
	assert.Equal(t, int32(10), capture.Value)
	assert.Nil(t, slotNamed(f, "x"))

	// the caller leaves the environment in the context register, so that is where it is read from
	lambda := requireFunc(t, funcs, "main.func0")
	ptr := requireClosurePtr(t, lambda)
	assert.Equal(t, NewReg(testTarget.ClosureContext), ptr.Loc)
	assert.Empty(t, ptr.Args, "the context register is a location, not a computed value")
	assert.Same(t, lambda.Entry.Values[0], ptr, "the environment must be read before any call can clobber it")

	// x's value sits right at the front of the environment, one hop away
	value := requireLoadFrom(t, lambda, ptr)
	assert.Zero(t, value.Offset)
	assert.True(t, types.Equal(types.Int(), value.Type))

	adds := findValues(lambda.Entry.Values, OpAdd)
	require.Len(t, adds, 1)
	assert.Contains(t, adds[0].Args, value, "the captured value is what feeds the addition")
}

func TestGenSsa_Closure_MutatedCaptureIsBoxed(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; let f = fun (y int) -> int { return x + y; }; x = 5; return x; }`)

	f := requireFunc(t, funcs, "main")
	env := requireEnvSlot(t, f)

	// x is written after the closure is built, so the environment holds a pointer to it
	assert.True(t, types.Equal(types.Tuple([]*types.Type{types.Pointer(types.Int())}), env.Type),
		"expected (*int,), got %v", env.Type)

	// that pointer is x's address rather than its value, so both sides see the same storage
	capture := requireStoredAt(t, f, env, 0)
	require.Equal(t, OpLocalAddr, capture.Op)
	assert.Same(t, slotNamed(f, "x"), capture.Slot())
	assert.True(t, types.Equal(types.Pointer(types.Int()), capture.Type))

	// one hop to the box pointer sitting at the front of the environment
	lambda := requireFunc(t, funcs, "main.func0")
	box := requireLoadFrom(t, lambda, requireClosurePtr(t, lambda))
	assert.Zero(t, box.Offset)
	assert.True(t, types.Equal(types.Pointer(types.Int()), box.Type))

	// and a second to x itself, which is what keeps a write on either side visible to both
	value := requireLoadFrom(t, lambda, box)
	assert.Zero(t, value.Offset)
	assert.True(t, types.Equal(types.Int(), value.Type))

	adds := findValues(lambda.Entry.Values, OpAdd)
	require.Len(t, adds, 1)
	assert.Contains(t, adds[0].Args, value, "the captured value is what feeds the addition")
}

func TestGenSsa_LetRec_SelfCaptureIsBoxed(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let rec g = fun () -> int { return g(); }; return 1; }`)

	f := requireFunc(t, funcs, "main")
	env := requireEnvSlot(t, f)

	// g is never written, but it has no value to copy until this very environment is built, so it stays boxed
	gType := types.Function(nil, types.Int())
	assert.True(t, types.Equal(types.Tuple([]*types.Type{types.Pointer(gType)}), env.Type),
		"expected (*fun () -> int,), got %v", env.Type)
}

// TestGenSsa_Closure_NestedCaptureForwardsWhatItWasHanded covers a middle lambda that never had x on its frame
func TestGenSsa_Closure_NestedCaptureForwardsWhatItWasHanded(t *testing.T) {
	tests := []struct {
		name  string
		write string // what main does to x once the closures are built
		want  *types.Type
	}{
		{name: "the value of an immutable capture", want: types.Int()},
		{name: "the box of a mutated capture", write: `x = 11;`, want: types.Pointer(types.Int())},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := requireBuildSSA(t, fmt.Sprintf(`fun main () -> int {
				let x = 10;
				let f = fun () -> int {
					let g = fun () -> int { return x; };
					return 2;
				};
				%s
				return 3;
			}`, tt.write))

			middle := requireFunc(t, funcs, "main.func0")
			handed := requireLoadFrom(t, middle, requireClosurePtr(t, middle))
			assert.True(t, types.Equal(tt.want, handed.Type))

			inner := requireStoredAt(t, middle, requireEnvSlot(t, middle), 0)
			assert.Same(t, handed, inner, "the inner environment must hold what the middle one was handed")
		})
	}
}

func TestGenSsa_Closure_CaptureAtLeafCapIsCopied(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int {
		let x = (1, 2, 3, 4);
		let f = fun () -> int { return x.0; };
		return 0;
	}`)

	f := requireFunc(t, funcs, "main")
	env := requireEnvSlot(t, f)

	// four leaves is still cheap enough to copy, so the environment holds x itself
	xType := types.Tuple([]*types.Type{types.Int(), types.Int(), types.Int(), types.Int()})
	assert.True(t, types.Equal(types.Tuple([]*types.Type{xType}), env.Type),
		"expected ((int, int, int, int),), got %v", env.Type)

	// and x needs no storage of its own
	assert.Nil(t, slotNamed(f, "x"))
}

func TestGenSsa_Closure_CaptureOverLeafCapIsBoxed(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int {
		let x = (1, 2, 3, 4, 5);
		let f = fun () -> int { return x.0; };
		return 0;
	}`)

	f := requireFunc(t, funcs, "main")
	env := requireEnvSlot(t, f)

	// x is never written, but five leaves is too many to copy on every call, so the environment points at it
	xType := types.Tuple([]*types.Type{types.Int(), types.Int(), types.Int(), types.Int(), types.Int()})
	assert.True(t, types.Equal(types.Tuple([]*types.Type{types.Pointer(xType)}), env.Type),
		"expected (*(int, int, int, int, int),), got %v", env.Type)

	// that pointer is x's address, so x keeps its storage
	capture := requireStoredAt(t, f, env, 0)
	require.Equal(t, OpLocalAddr, capture.Op)
	assert.Same(t, slotNamed(f, "x"), capture.Slot())
}

// requireEnvSlot returns the single closure environment f builds.
func requireEnvSlot(t *testing.T, f *Func) *Slot {
	t.Helper()

	var found []*Slot
	for _, s := range f.Slots {
		if s.Sym == nil && s.Type.IsTuple() {
			found = append(found, s)
		}
	}

	require.Len(t, found, 1)
	return found[0]
}

// requireClosurePtr returns the value naming the environment f was called with.
func requireClosurePtr(t *testing.T, f *Func) *Value {
	t.Helper()

	ptrs := findValues(f.Entry.Values, OpClosurePtr)
	require.Len(t, ptrs, 1)
	return ptrs[0]
}

// requireStoredAt returns the value written to slot at the given byte offset.
func requireStoredAt(t *testing.T, f *Func, slot *Slot, offset int) *Value {
	t.Helper()

	var found []*Value
	for v := range f.UnorderedValues() {
		if v.Op == OpStaticStore && v.Slot() == slot && v.Offset == offset {
			found = append(found, v)
		}
	}

	require.Len(t, found, 1, "expected one store at offset %d", offset)
	require.Len(t, found[0].Args, 1)
	return found[0].Args[0]
}

// throughCopies follows v back past any copies to the value that produced it.
func throughCopies(v *Value) *Value {
	for v.Op == OpCopy {
		v = v.Args[0]
	}
	return v
}

// requireLoadFrom returns the single value read through ptr.
func requireLoadFrom(t *testing.T, f *Func, ptr *Value) *Value {
	t.Helper()

	var found []*Value
	for v := range f.UnorderedValues() {
		if v.Op == OpLoad && len(v.Args) > 0 && v.Args[0] == ptr {
			found = append(found, v)
		}
	}

	require.Len(t, found, 1)
	return found[0]
}

func TestGenSsa_ClosureCall_OperandOrder(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int {
		let x = 10;
		let f = fun (y int, z int) -> int { return x + y + z; };
		return f(2, 3);
	}`)

	f := requireFunc(t, funcs, "main")
	call := requireClosureCall(t, funcs, "main")

	// the code pointer and the environment are the only operands ahead of the two ABI arguments
	require.Len(t, call.Args, 4)
	require.Len(t, call.CallArgs(), 2, "the leading operands are not arguments")

	// they travel out of band, so the arguments start at the first arg register
	assertInArgRegisters(t, call.CallArgs())

	// the first operand names the lifted lambda directly
	code := throughCopies(call.Args[0])
	require.Equal(t, OpLabelAddr, code.Op)
	assert.Equal(t, "main.func0", code.Callee().Name())

	// the callee reads its captures out of the context register, so the environment has to land there
	assert.Equal(t, OpCopy, call.Args[1].Op, "the environment must be placed by a copy into its register")
	assert.Equal(t, NewReg(testTarget.ClosureContext), call.Args[1].Loc)

	// the second is the environment, which the unknown target can't leak, so it stays on the frame
	env := throughCopies(call.Args[1])
	require.Equal(t, OpLocalAddr, env.Op)
	assert.Same(t, requireEnvSlot(t, f), env.Slot())

	// the code pointer travels in the closure itself, so nothing loads it out of the environment
	assert.Empty(t, findValues(f.Entry.Values, OpLoad),
		"the code pointer must not be read through memory")
}

// requireClosureCall returns the single indirect call in the named function.
func requireClosureCall(t *testing.T, funcs []*Func, funcName string) *Value {
	t.Helper()

	calls := findValues(requireFunc(t, funcs, funcName).Entry.Values, OpClosureCall)
	require.Len(t, calls, 1)
	return calls[0]
}

func TestGenSsa_DirectCall_BuildsNoEnvironment(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun add (x int, y int) -> int { return x + y; }
		fun main () -> int { return add(1, 2); }`)

	f := requireFunc(t, funcs, "main")

	// naming a global as a call target is still a direct call, with nothing allocated for it
	call := requireCall(t, funcs, "main")
	require.NotNil(t, call.Callee(), "call should name its target")
	assert.Equal(t, "add", call.Callee().Name())

	// a direct call names its target, so Args holds the arguments alone
	assert.Len(t, call.Args, 2)

	assert.Empty(t, findValues(f.Entry.Values, OpClosureCall))
	assert.Empty(t, findValues(f.Entry.Values, OpLabelAddr))
	assert.Empty(t, f.Slots)
}

func TestSpill_EvictedLiteralIsRebuiltAtItsUse(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int {
		let t = (0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0);
		let p = &t;
		*p = (1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16);
		return t.0 + t.15;
	}`)

	f := requireFunc(t, funcs, "main")
	values := f.Entry.Values

	// every field is built before the first store, so more literals are live at once than there are registers;
	// the ones that lose theirs are rebuilt in among the stores that need them
	firstStore := slices.IndexFunc(values, func(v *Value) bool { return v.Op == OpStore })
	require.GreaterOrEqual(t, firstStore, 0)

	lastLiteral := -1
	for i, v := range values {
		if v.Op == OpLiteral {
			lastLiteral = i
		}
	}
	assert.Greater(t, lastLiteral, firstStore, "an evicted literal must be rebuilt where a later store reads it")

	// rather than taking a round trip through a spill slot
	for _, v := range values {
		if v.Op == OpStaticStore && v.Slot().Sym == nil {
			assert.NotEqual(t, OpLiteral, v.Args[0].Op, "a literal must never be spilled")
		}
	}

	// and a literal rebuilt at every use leaves nothing reading its first definition, so that definition goes too
	assert.Empty(t, slices.DeleteFunc(findValues(values, OpLiteral), f.hasUses))
}

// TestPruneUnreachable_EveryPathReturns covers conditionals that strand their merge block behind returns
func TestPruneUnreachable_EveryPathReturns(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "as trailing value", src: `fun f (b bool) -> int { if b { return 1; } else { return 2; } }`},
		{name: "as last statement", src: `fun f (b bool) -> int { if b { return 1; } else { return 2; }; }`},
		{name: "through elif", src: `fun f (n int) -> int { if n < 0 { return 0; } elif n == 0 { return 1; } else { return 2; }; }`},
		{
			// the dead inner merge still jumps to the outer merge, so it would linger as a predecessor
			name: "nested in branch",
			src:  `fun f (a bool, b bool) -> int { if a { if b { return 1; } else { return 2; } } else { 3 } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, tt.src), "f")

			reachable := blockIds(slices.Collect(f.OrderedBlocks()))
			assert.ElementsMatch(t, reachable, blockIds(f.Blocks), "every remaining block is reachable")

			for _, b := range f.Blocks {
				assert.Subset(t, reachable, blockIds(b.Predecessors), "block %d has an unreachable predecessor", b.Id)
			}
		})
	}
}

func TestGenSsa_Conditional_BuildsDiamond(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `fun f (c bool) -> int { if c { 1 } else { 2 } }`), "f")

	branch := f.Entry
	require.Equal(t, BlockIf, branch.Kind)
	require.Len(t, branch.Control, 1)
	assert.Equal(t, types.Bool(), branch.Control[0].Type)
	require.Len(t, branch.Successors, 2)

	// both sides fall into the same merge block, which returns
	then, els := branch.Successors[0], branch.Successors[1]
	for _, side := range []*Block{then, els} {
		assert.Equal(t, BlockPlain, side.Kind)
		assert.Equal(t, []int{branch.Id}, blockIds(side.Predecessors))
		require.Len(t, side.Successors, 1)
	}

	merge := then.Successors[0]
	assert.Same(t, merge, els.Successors[0])
	assert.Equal(t, BlockRet, merge.Kind)
	require.Equal(t, []int{then.Id, els.Id}, blockIds(merge.Predecessors))

	phi := requireReturned(t, merge)
	require.Equal(t, OpPhi, phi.Op, "the merge block joins the value each arm leaves")
	require.Len(t, phi.Args, 2)

	// each operand lines up with the predecessor it comes in from
	assert.Same(t, then, phi.Args[0].Block)
	assert.Equal(t, OpLiteral, phi.Args[0].Op)
	assert.Equal(t, int32(1), phi.Args[0].Value)
	assert.Same(t, els, phi.Args[1].Block)
	assert.Equal(t, OpLiteral, phi.Args[1].Op)
	assert.Equal(t, int32(2), phi.Args[1].Value)

	assert.Empty(t, findValues(then.Values, OpStaticStore), "the slot is promoted out of memory")
	assert.Empty(t, findValues(els.Values, OpStaticStore), "the slot is promoted out of memory")
	assert.Empty(t, findValues(merge.Values, OpStaticLoad), "the slot is promoted out of memory")
}

func TestGenSsa_Loop_BreakInConditionalExitsLoop(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (n int) -> int { let i = 0; loop { if i == n { break; }; i = i + 1; }; i }`), "f")

	require.Len(t, f.Entry.Successors, 1)
	header := f.Entry.Successors[0]
	require.Equal(t, BlockIf, header.Kind)
	require.Len(t, header.Successors, 2)

	// the breaking arm leaves the loop entirely rather than falling into the conditional's merge
	then := header.Successors[0]
	require.Len(t, then.Successors, 1)
	exit := then.Successors[0]
	assert.Equal(t, BlockRet, exit.Kind)
	assert.Equal(t, []int{then.Id}, blockIds(exit.Predecessors))

	// the other arm carries on through the rest of the body and back around
	els := header.Successors[1]
	require.Len(t, els.Successors, 1)
	merge := els.Successors[0]
	require.Len(t, merge.Successors, 1)
	assert.Same(t, header, merge.Successors[0])
}

func TestGenSsa_Loop_ContinueInConditionalTargetsHeader(t *testing.T) {
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (n int) -> int { let i = 0; loop { i = i + 1; if i < n { continue; }; break; }; i }`), "f")

	require.Len(t, f.Entry.Successors, 1)
	header := f.Entry.Successors[0]
	require.Equal(t, BlockIf, header.Kind)
	require.Len(t, header.Successors, 2)

	then := header.Successors[0]
	require.Len(t, then.Successors, 1)
	assert.Same(t, header, then.Successors[0])
}

func TestGenSsa_Loop_BodyValueIsEvaluated(t *testing.T) {
	// the body's unit value is discarded, but the call producing it still happens
	f := requireFunc(t, requireBuildSSA(t, `
		fun g () { }
		fun f () { loop { g() }; }`), "f")

	assert.Len(t, findValues(slices.Collect(f.OrderedValues()), OpStaticCall), 1)
}

func TestMem2Reg_VariableReadAcrossABranchIsPromoted(t *testing.T) {
	// neither arm assigns y, so every path reaching the read agrees on its value
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool, x int) -> int { let y = x * 3; if c { 1 } else { 2 }; y }`), "f")

	assert.Nil(t, slotNamed(f, "y"), "y should be promoted out of the frame")
	assert.Empty(t, findValues(slices.Collect(f.OrderedValues()), OpStaticLoad), "nothing is read back out of memory")

	merge := f.Entry.Successors[0].Successors[0]
	ret := requireReturned(t, merge)
	assert.Equal(t, OpMultiply, ret.Op)
	assert.Same(t, f.Entry, ret.Block, "the merge reads the value worked out before the branch")
}

func TestMem2Reg_VariableReassignedInsideOneArmIsPromoted(t *testing.T) {
	// y changes in the then arm, but only that arm reads the new value
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool, x int) -> int { let y = x * 3; if c { y = y + 1; return y; }; 0 }`), "f")

	assert.Nil(t, slotNamed(f, "y"), "y should be promoted out of the frame")

	then := f.Entry.Successors[0]
	ret := requireReturned(t, then)
	require.Equal(t, OpAdd, ret.Op)
	require.Len(t, ret.Args, 2)
	assert.Equal(t, OpMultiply, ret.Args[0].Op, "the arm reads the value stored before the branch")
}

func TestMem2Reg_ArmThatReturnsLeavesNothingToMerge(t *testing.T) {
	// only the else arm reaches the merge, so the conditional's value comes from it alone
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool) -> int { let x = if c { return 0; } else { 2 }; x }`), "f")

	assert.Empty(t, f.Slots, "the conditional's value and x should both be promoted")

	merge := f.Entry.Successors[1].Successors[0]
	ret := requireReturned(t, merge)
	assert.Equal(t, OpLiteral, ret.Op)
	assert.Equal(t, int32(2), ret.Value)
}

// TestSpill_ValueReadAcrossABranchKeepsItsRegister covers a value read before a branch and read again once it merges
func TestSpill_ValueReadAcrossABranchKeepsItsRegister(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "both arms merge", src: `fun f (c bool, x int) -> int { (x - 1) + (x - if c { 1 } else { 2 }) }`},
		{
			// only the arm that reaches the merge decides whether x is still needed
			name: "one arm returns",
			src:  `fun f (c bool, x int) -> int { (x - 1) + (x - if c { return 0; } else { 2 }) }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, tt.src), "f")

			subs := findValues(slices.Collect(f.OrderedValues()), OpSubtract)
			require.Len(t, subs, 2)
			require.Same(t, f.Entry, subs[0].Block, "x - 1 is worked out before the branch")
			require.NotSame(t, f.Entry, subs[1].Block, "the other subtract has to wait for the branch")

			// x is still needed past the branch, so its register can't be handed back after the first read
			assert.Same(t, subs[0].Args[0], subs[1].Args[0], "x should be read straight out of its register, not reloaded")
		})
	}
}

func TestSpill_ValueEvictedInsideABranchIsStoredWhereItIsDefined(t *testing.T) {
	// a through n wait out the branch, and the then arm needs more registers than they leave free
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool, a int, b int, d int, e int, g int, h int, i int, j int, k int, l int, m int, n int) -> int {
			a + (b + (d + (e + (g + (h + (i + (j + (k + (l + (m + (n + if c { a * b + (d * e + (g * h)) } else { 1 })))))))))))
		}`), "f")
	require.Len(t, f.Entry.Successors, 2)
	then := f.Entry.Successors[0]

	// the then arm is where registers run out, but a store there would only run on one path to the merge
	assert.Empty(t, findStoresWhere(then, func(v *Value) bool { return v.Args[0].Block != then }))

	// so what it gives up is stored where it was defined, which runs whichever way the branch goes
	assert.NotEmpty(t, findStoresWhere(f.Entry, func(v *Value) bool { return v.Slot().Sym == nil }))

	// the condition is dead once the branch is taken, so the arms never hold it, let alone spill it
	require.Equal(t, BlockIf, f.Entry.Kind)
	require.Len(t, f.Entry.Control, 1)
	assert.Empty(t, findStoresOf(f, f.Entry.Control[0]))
}

func TestSpill_PressureBeforeABranchGivesUpTheValueReadLast(t *testing.T) {
	// a through q wait out the branch alongside the condition, more than fit in registers;
	// once the arms merge, q is read first and a last
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool, a int, b int, d int, e int, g int, h int, i int, j int, k int, l int, m int, n int, o int, p int, q int) -> int {
			a + (b + (d + (e + (g + (h + (i + (j + (k + (l + (m + (n + (o + (p + (q + if c { 1 } else { 2 }))))))))))))))
		}`), "f")

	// counting through the branch, a's next read is the furthest away, so it's the one spilled
	assert.Len(t, findStoresOf(f, requireParamCopy(t, f, 1)), 1)

	// q is read first after the branch, so it keeps its register
	assert.Empty(t, findStoresOf(f, requireParamCopy(t, f, 15)))
}

func TestSpill_EvictedConditionIsReloadedForTheBranch(t *testing.T) {
	// c is defined first but only read by the branch, so it's the furthest read while s is worked out
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool, a int, b int, d int, e int, g int, h int, i int, j int, k int, l int, m int, n int, o int, p int, q int) -> int {
			let s = a * (b + (d * (e + (g * (h + (i * (j + (k * (l + (m * (n + (o * (p + q)))))))))))));
			if c { s } else { 0 }
		}`), "f")
	require.Equal(t, BlockIf, f.Entry.Kind)
	require.Len(t, f.Entry.Control, 1)

	stores := findStoresOf(f, requireParamCopy(t, f, 0))
	require.Len(t, stores, 1, "c gives up its register while s is worked out")

	// the branch reads c back out of the slot it was stored to, not the register it gave up
	cond := f.Entry.Control[0]
	assert.Equal(t, OpStaticLoad, cond.Op)
	assert.Same(t, stores[0].Slot(), cond.Slot())
	assert.Same(t, f.Entry, cond.Block)
}

func TestRegalloc_ValueOnlyOneArmNeedsIgnoresACallInTheOther(t *testing.T) {
	// x escapes, so the entry block allocates it and only the then arm still needs the pointer
	f := requireFunc(t, requireBuildSSA(t, `
		fun g () -> int { 0 }
		fun f (c bool) -> *int { let x = 7; if c { return &x; } else { g(); return nil; } }`), "f")
	require.Len(t, f.Entry.Successors, 2)
	then, els := f.Entry.Successors[0], f.Entry.Successors[1]
	require.NotEmpty(t, findValues(els.Values, OpStaticCall), "the else arm makes a call")

	ptr := requireReturned(t, then)
	require.Same(t, f.Entry, ptr.Block, "the pointer is made before the branch")

	// the call clobbers caller-saved registers, but only on a path where the pointer is already dead
	require.Equal(t, LocRegister, ptr.Loc.Kind)
	assert.True(t, testTarget.CallerSaved.Contains(ptr.Loc.Reg), "expected a caller-saved register, got %v", ptr.Loc.Reg)
}

func TestSpill_EvictedPhiIsStoredAfterEveryPhi(t *testing.T) {
	// a through n wait out the branch alongside both phis, just filling every register as the merge starts,
	// so the 2 it starts with needs one given up; y is read last, so its phi goes
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool, a int, b int, d int, e int, g int, h int, i int, j int, k int, l int, m int, n int) -> int {
			let x = 0;
			let y = 0;
			if c { x = 1; y = 2; } else { x = 3; y = 4; };
			y + (a + (b + (d + (e + (g + (h + (i + (j + (k + (l + (m + (n + x * 2))))))))))))
		}`), "f")
	require.Len(t, f.Entry.Successors, 2)
	then := f.Entry.Successors[0]
	require.Len(t, then.Successors, 1)
	merge := then.Successors[0]
	require.GreaterOrEqual(t, len(merge.Values), 3)

	stores := findStoresOf(f, merge.Values[0])
	require.Len(t, stores, 1, "the first phi is given up")

	// phis sit together at the top of the block, so the store waits until the last of them
	assert.Equal(t, OpPhi, merge.Values[0].Op)
	assert.Equal(t, OpPhi, merge.Values[1].Op)
	assert.Same(t, stores[0], merge.Values[2])
}

func TestSpill_PhisBeyondTheRegistersWaitInMemory(t *testing.T) {
	// all sixteen are merged, and every phi takes its register as the merge starts, so not all of them can
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (c bool) -> int {
			let a = 0; let b = 0; let d = 0; let e = 0; let g = 0; let h = 0; let i = 0; let j = 0;
			let k = 0; let l = 0; let m = 0; let n = 0; let o = 0; let p = 0; let q = 0; let r = 0;
			if c {
				a = 1; b = 2; d = 3; e = 4; g = 5; h = 6; i = 7; j = 8;
				k = 9; l = 10; m = 11; n = 12; o = 13; p = 14; q = 15; r = 16;
			} else {
				a = 17; b = 18; d = 19; e = 20; g = 21; h = 22; i = 23; j = 24;
				k = 25; l = 26; m = 27; n = 28; o = 29; p = 30; q = 31; r = 32;
			};
			a + b + d + e + g + h + i + j + k + l + m + n + o + p + q + r
		}`), "f")
	require.Len(t, f.Entry.Successors, 2)
	then, els := f.Entry.Successors[0], f.Entry.Successors[1]
	merge := requireReturnBlock(t, f)

	assert.LessOrEqual(t, len(findValues(merge.Values, OpPhi)), testTarget.Allocatable.Count(), "only as many phis as there are registers")

	sum := requireReturned(t, merge)
	require.Equal(t, OpAdd, sum.Op)
	require.Len(t, sum.Args, 2)

	// r is read last, so it's one of the phis that waits in memory, and the merge reads it back from its slot
	last := sum.Args[1]
	require.Equal(t, OpStaticLoad, last.Op)
	assert.Same(t, merge, last.Block)

	// each arm leaves its value for r in that slot on the way out
	thenStores := findStoresTo(then, last.Slot())
	require.Len(t, thenStores, 1)
	assert.Equal(t, OpLiteral, thenStores[0].Args[0].Op)
	assert.Equal(t, int32(16), thenStores[0].Args[0].Value)

	elseStores := findStoresTo(els, last.Slot())
	require.Len(t, elseStores, 1)
	assert.Equal(t, OpLiteral, elseStores[0].Args[0].Op)
	assert.Equal(t, int32(32), elseStores[0].Args[0].Value)

	// every arm's literals outnumber the registers, so some are rebuilt to be stored for a memory phi;
	// once that phi is gone, nothing reads the literal it was handed, so that literal goes too
	assert.Empty(t, slices.DeleteFunc(findValues(then.Values, OpLiteral), f.hasUses))
	assert.Empty(t, slices.DeleteFunc(findValues(els.Values, OpLiteral), f.hasUses))
}

func TestRegalloc_PhiTakesTheRegisterItsOperandsShare(t *testing.T) {
	// x outlives a call in each arm, so both arms keep it in the same callee-saved register,
	// away from the result register the phi would otherwise be drawn to
	f := requireFunc(t, requireBuildSSA(t, `
		fun g (v int) -> int { v }
		fun f (c bool) -> int {
			let x = 0;
			if c { x = 1; g(x); } else { x = 2; g(x); };
			x
		}`), "f")
	require.Len(t, f.Entry.Successors, 2)
	then := f.Entry.Successors[0]
	require.Len(t, then.Successors, 1)

	phi := requireReturned(t, then.Successors[0])
	require.Equal(t, OpPhi, phi.Op)
	require.Len(t, phi.Args, 2)
	require.Equal(t, LocRegister, phi.Args[0].Loc.Kind)
	require.True(t, testTarget.CalleeSaved.Contains(phi.Args[0].Loc.Reg))
	require.Equal(t, phi.Args[0].Loc, phi.Args[1].Loc)

	// so the phi joins them, and neither edge has to move anything
	assert.Equal(t, phi.Args[0].Loc, phi.Loc)
}

// TestGenSsa_ShortCircuit_EvaluatesRightOperandOnOneArm relies on the first successor being taken when the condition holds
func TestGenSsa_ShortCircuit_EvaluatesRightOperandOnOneArm(t *testing.T) {
	tests := []struct {
		op    string
		right int // the successor that evaluates the right operand
	}{
		{op: "&&", right: 0},
		{op: "||", right: 1},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			src := fmt.Sprintf(`fun g () -> bool { true } fun f (a bool) -> bool { a %s g() }`, tt.op)
			branch := requireFunc(t, requireBuildSSA(t, src), "f").Entry
			require.Equal(t, BlockIf, branch.Kind)
			require.Len(t, branch.Successors, 2)

			assert.Empty(t, findValues(branch.Values, OpStaticCall))
			assert.Len(t, findValues(branch.Successors[tt.right].Values, OpStaticCall), 1)
			assert.Empty(t, findValues(branch.Successors[1-tt.right].Values, OpStaticCall))
		})
	}
}

// TestMem2Reg_ShortCircuit_JoinsRightOperandWithLiteral covers the phi merging the two ways out of && and ||
func TestMem2Reg_ShortCircuit_JoinsRightOperandWithLiteral(t *testing.T) {
	tests := []struct {
		op      string
		right   int  // the phi operand holding the right operand
		literal bool // what the arm skipping the right operand leaves
	}{
		{op: "&&", right: 0, literal: false},
		{op: "||", right: 1, literal: true},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			f := requireFunc(t, requireBuildSSA(t, fmt.Sprintf(`fun f (a bool, b bool) -> bool { a %s b }`, tt.op)), "f")

			require.Len(t, f.Entry.Successors, 2)
			then := f.Entry.Successors[0]
			require.Len(t, then.Successors, 1)

			phi := requireReturned(t, then.Successors[0])
			require.Equal(t, OpPhi, phi.Op)
			require.Len(t, phi.Args, 2)

			assert.Same(t, requireParamCopy(t, f, 1), phi.Args[tt.right])

			skipped := phi.Args[1-tt.right]
			assert.Same(t, f.Entry.Successors[1-tt.right], skipped.Block)
			assert.Equal(t, OpLiteral, skipped.Op)
			assert.Equal(t, tt.literal, skipped.Value)
		})
	}
}

func TestSpill_ValueNothingReadsAgainIsNotStored(t *testing.T) {
	// the loop reads more values than there are registers, so each reload feeding an add is given up right after it
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (n int) -> int {
			let a = n + 1; let b = n + 2; let c = n + 3; let d = n + 4; let e = n + 5; let g = n + 6;
			let h = n + 7; let j = n + 8; let k = n + 9; let l = n + 10; let m = n + 11; let o = n + 12;
			let p = n + 13; let q = n + 14; let r = n + 15; let s = n + 16;
			let i = 0;
			let acc = 0;
			loop {
				if i == 2 { break; };
				acc = acc + a + b + c + d + e + g + h + j + k + l + m + o + p + q + r + s;
				i = i + 1;
			};
			acc + a + s
		}`), "f")

	assert.Empty(t, findUnreadStores(f))
}

func TestSpill_BackEdgeRestoresValueEvictedInTheBody(t *testing.T) {
	// the body reads every value before its call, then gives one up to keep s across the call
	f := requireFunc(t, requireBuildSSA(t, `
		fun id (x int) -> int { x }
		fun f () -> int {
			let a = id(1); let b = id(2); let c = id(3); let d = id(4); let e = id(5); let g = id(6); let h = id(7);
			let s = 0;
			let i = 0;
			loop {
				if i == 3 { break; };
				s = s + a + b + c + d + e + g + h;
				s = s + id(0);
				i = i + 1;
			};
			s
		}`), "f")

	latch := requireLatch(t, f)
	restores := findRestores(latch)
	require.Len(t, restores, 1)
	restore := restores[0]

	// it comes back from the slot it was stored in, to the register the header expects it in
	require.Equal(t, OpStaticLoad, restore.Op)
	assert.Equal(t, requireStoreTo(t, f, restore.Slot()).Args[0].Loc, restore.Loc)

	// s and i go around the same edge, so each phi has its operand in place as the latch starts,
	// so the latch's body can write what the moves read
	header := latch.Successors[0]
	back := slices.Index(header.Predecessors, latch)
	phis := header.Values[:header.phiIndex()]
	require.Len(t, phis, 2)
	for _, phi := range phis {
		assert.Equal(t, OpPhi, phi.Args[back].Op)
		assert.Same(t, latch, phi.Args[back].Block)
		assert.Equal(t, phi.Loc, phi.Args[back].Loc)
	}
}

func TestSpill_BackEdgeRebuildsLiteralEvictedInTheBody(t *testing.T) {
	// literals are the cheapest thing to give up, so the one read at the top of the body goes first
	f := requireFunc(t, requireBuildSSA(t, `
		fun id (x int) -> int { x }
		fun f () -> int {
			let k = 7;
			let s = 0;
			let i = 0;
			loop {
				if i == 3 { break; };
				s = s + k;
				let a = id(1); let b = id(2); let c = id(3); let d = id(4); let e = id(5); let g = id(6);
				s = s + a + b + c + d + e + g;
				i = i + 1;
			};
			s + k
		}`), "f")

	restores := findRestores(requireLatch(t, f))
	require.Len(t, restores, 1)
	restore := restores[0]

	assert.Equal(t, OpLiteral, restore.Op)
	assert.Equal(t, int32(7), restore.Value)
	assert.Equal(t, requireLiteral(t, f.Entry, int32(7)).Loc, restore.Loc)
}

func TestSpill_MemoryPhiTakesItsOperandOnTheBackEdge(t *testing.T) {
	// more values are carried around the loop than there are registers, so r waits in memory
	f := requireFunc(t, requireBuildSSA(t, `
		fun f (n int) -> int {
			let a = 0; let b = 0; let d = 0; let e = 0; let g = 0; let h = 0; let j = 0; let k = 0;
			let l = 0; let m = 0; let o = 0; let p = 0; let q = 0; let r = 0; let s = 0; let u = 0;
			let i = 0;
			loop {
				if i == n { break; };
				a = a + 1; b = b + 2; d = d + 3; e = e + 4; g = g + 5; h = h + 6; j = j + 7; k = k + 8;
				l = l + 9; m = m + 10; o = o + 11; p = p + 12; q = q + 13; r = r + 14; s = s + 15; u = u + 16;
				i = i + 1;
			};
			a + b + d + e + g + h + j + k + l + m + o + p + q + r + s + u
		}`), "f")
	latch := requireLatch(t, f)
	exit := requireReturnBlock(t, f)

	sum := requireReturned(t, exit)
	require.Equal(t, OpAdd, sum.Op)
	require.Len(t, sum.Args, 2)
	last := sum.Args[1]
	require.Equal(t, OpStaticLoad, last.Op)

	// each trip around the loop leaves the next value in the slot the exit reads, before the edge puts anything back
	require.Len(t, latch.Predecessors, 1)
	stores := findStoresTo(latch.Predecessors[0], last.Slot())
	require.Len(t, stores, 1)
	assert.Equal(t, OpAdd, stores[0].Args[0].Op)
}

func TestSpill_BackEdgeLoadsSpilledOperandIntoItsPhi(t *testing.T) {
	// total's next value is computed first and read last, so it waits out the calls in a slot
	f := requireFunc(t, requireBuildSSA(t, `
		fun id (x int) -> int { x }
		fun f (n int) -> int {
			let total = 0;
			let i = 0;
			loop {
				if i == n { break; };
				total = total + 1;
				let a = id(1); let b = id(2); let c = id(3); let d = id(4); let e = id(5); let g = id(6);
				i = i + a + b + c + d + e + g;
			};
			total
		}`), "f")
	latch := requireLatch(t, f)

	operand := requireRestoredOperand(t, latch)

	assert.Equal(t, OpStaticLoad, operand.Op)
	assert.Same(t, latch, operand.Block)
	assert.Equal(t, OpAdd, requireStoreTo(t, f, operand.Slot()).Args[0].Op)
}

func TestSpill_BlockJumpingToItselfFinishesItsOwnEdge(t *testing.T) {
	// with no way out the whole body is one block, which is its own header and its own latch
	f := requireFunc(t, requireBuildSSA(t, `
		fun id (x int) -> int { x }
		fun f () {
			let a = id(1); let b = id(2); let c = id(3); let d = id(4); let e = id(5); let g = id(6); let h = id(7);
			let s = 0;
			loop {
				s = s + a + b + c + d + e + g + h;
				s = s + id(0);
			};
		}`), "f")
	latch := requireLatch(t, f)

	// what the edge puts back gets a block of its own, between the body and itself
	require.Len(t, latch.Predecessors, 1)
	assert.Same(t, latch.Predecessors[0], latch.Successors[0])
	assert.NotEmpty(t, findRestores(latch))
}

func blockIds(blocks []*Block) []int {
	ids := make([]int, len(blocks))
	for i, b := range blocks {
		ids[i] = b.Id
	}
	return ids
}

// requireLatch returns the one block jumping back to the header of f's one loop
func requireLatch(t *testing.T, f *Func) *Block {
	t.Helper()

	loops := f.Loops()
	var latches []*Block
	for _, b := range f.Blocks {
		if l := loops[b]; l != nil && slices.Contains(b.Successors, l.Header) {
			latches = append(latches, b)
		}
	}
	require.Len(t, latches, 1)
	return latches[0]
}

// requireReturnBlock returns the one block f returns from
func requireReturnBlock(t *testing.T, f *Func) *Block {
	t.Helper()

	var exits []*Block
	for _, b := range f.Blocks {
		if b.Kind == BlockRet {
			exits = append(exits, b)
		}
	}
	require.Len(t, exits, 1)
	return exits[0]
}

// findRestores returns what the edge block b puts back in registers, which is everything in it past its phis
func findRestores(b *Block) []*Value {
	return b.Values[b.phiIndex():]
}

// requireRestoredOperand returns the one restore in b that hands a phi of b's successor its operand
func requireRestoredOperand(t *testing.T, b *Block) *Value {
	t.Helper()

	var operands []*Value
	for _, arg := range b.Successors[0].PhiArgsFrom(b) {
		if arg.Op != OpPhi && arg.Block == b {
			operands = append(operands, arg)
		}
	}
	require.Len(t, operands, 1)
	return operands[0]
}

// findStoresOf returns every store in f that writes v to a slot
func findStoresOf(f *Func, v *Value) []*Value {
	var result []*Value
	for u := range f.UnorderedValues() {
		if u.Op == OpStaticStore && u.Args[0] == v {
			result = append(result, u)
		}
	}
	return result
}

// requireStoreTo returns the one store in f that writes to slot
func requireStoreTo(t *testing.T, f *Func, slot *Slot) *Value {
	t.Helper()

	stores := findValues(slices.Collect(f.SlotValues(slot)), OpStaticStore)
	require.Len(t, stores, 1)
	return stores[0]
}

// requireLiteral returns the one literal in b with the given value
func requireLiteral(t *testing.T, b *Block, value any) *Value {
	t.Helper()

	var literals []*Value
	for _, v := range findValues(b.Values, OpLiteral) {
		if v.Value == value {
			literals = append(literals, v)
		}
	}
	require.Len(t, literals, 1)
	return literals[0]
}

// findStoresTo returns the stores in b that write to slot
func findStoresTo(b *Block, slot *Slot) []*Value {
	return findStoresWhere(b, func(v *Value) bool { return v.Slot() == slot })
}

// findStoresWhere returns the stores in b that match
func findStoresWhere(b *Block, match func(*Value) bool) []*Value {
	var result []*Value
	for _, v := range findValues(b.Values, OpStaticStore) {
		if match(v) {
			result = append(result, v)
		}
	}
	return result
}

// findUnreadStores returns every store in f to a slot nothing in f loads from
func findUnreadStores(f *Func) []*Value {
	var result []*Value
	for v := range f.UnorderedValues() {
		if v.Op != OpStaticStore {
			continue
		}
		loaded := false
		for u := range f.SlotValues(v.Slot()) {
			loaded = loaded || u.Op == OpStaticLoad
		}
		if !loaded {
			result = append(result, v)
		}
	}
	return result
}

// requireParamCopy returns the copy f's entry block makes of its ith incoming parameter
func requireParamCopy(t *testing.T, f *Func, i int) *Value {
	t.Helper()

	var copies []*Value
	for _, v := range findValues(f.Entry.Values, OpCopy) {
		if v.Args[0].Op == OpParam && v.Args[0].Value == i {
			copies = append(copies, v)
		}
	}
	require.Len(t, copies, 1)
	return copies[0]
}

// assertInArgRegisters asserts that values sit in the System V argument registers, in order
func assertInArgRegisters(t *testing.T, values []*Value) {
	t.Helper()

	regs := []target.Register{target.RegDI, target.RegSI, target.RegD, target.RegC, target.Reg8, target.Reg9}
	for i, v := range values {
		assert.Equal(t, NewReg(regs[i]), v.Loc)
	}
}

func TestRegs_X64Divide(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun div (a int, b int) -> int { return a / b; }
		fun main () -> int { return div(7, 2); }
	`)

	divs := findValues(requireFunc(t, funcs, "div").Entry.Values, OpDivide)
	require.Len(t, divs, 1)
	regs := divs[0].regs()

	// the dividend is read out of rax, and the divisor from anywhere idiv's halves aren't
	require.Len(t, regs.inputs, 2)
	assert.Equal(t, target.NewMask(target.RegA), regs.inputs[0])
	assert.Equal(t, testTarget.Allocatable.Without(target.NewMask(target.RegA, target.RegD)), regs.inputs[1])

	// the quotient lands in rax, and sign-extending into rdx then leaving the remainder there wipes it out
	assert.Equal(t, target.NewMask(target.RegA), regs.output)
	assert.Equal(t, target.NewMask(target.RegD), regs.clobbers)
	assert.False(t, regs.twoAddress)
}

func TestRegs_CallClobbersCallerSaved(t *testing.T) {
	src := `
		fun callee (a int) -> int { return a; }
		fun main () -> int { return callee(7); }
	`

	for _, tgt := range []*target.Target{target.X64SysV, target.ARM64AAPCS, target.ARM64Apple} {
		call := requireCall(t, requireBuildSSAFor(t, tgt, src), "main")
		regs := call.regs()

		// a call leaves its results to OpCallResult, so it has no output of its own
		assert.Equal(t, tgt.CallerSaved, regs.clobbers)
		assert.Zero(t, regs.output)
	}

	// clobbers come from the calling convention, so they can name registers the allocator never hands out
	call := requireCall(t, requireBuildSSAFor(t, target.ARM64AAPCS, src), "main")
	assert.True(t, call.regs().clobbers.Contains(target.RegX16))
}

func TestBlockedAt_DivideClobbersRdxAsItRuns(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun div (a int, b int) -> int { return a / b; }
		fun main () -> int { return div(7, 2); }
	`)

	f := requireFunc(t, funcs, "div")
	divs := findValues(f.Entry.Values, OpDivide)
	require.Len(t, divs, 1)
	p := f.Entry.indexOf(divs[0])

	// the dividend's halves stop being live as idiv reads them, so only the clobber keeps rdx off limits there
	ranges := computeLiveRanges(f, computeNextUses(f))
	assert.True(t, ranges.blockedAt(f.Entry, p).Contains(target.RegD))
}

// requireOnly returns the single value with op in the named function's entry block
func requireOnly(t *testing.T, funcs []*Func, name string, op Op) *Value {
	t.Helper()
	vals := findValues(requireFunc(t, funcs, name).Entry.Values, op)
	require.Len(t, vals, 1)
	return vals[0]
}

// withX64Regs swaps op's entry in the x64 register table for the rest of the test
func withX64Regs(t *testing.T, op Op, info regInfo) {
	old, had := x64Regs[op]
	x64Regs[op] = info
	t.Cleanup(func() {
		if had {
			x64Regs[op] = old
		} else {
			delete(x64Regs, op)
		}
	})
}

// a demand of more than one register is left to the allocator rather than pinned by lowering
var twoCalleeSaved = target.NewMask(target.Reg13, target.Reg14)

func TestAllocate_ResultLandsWhereItsInstructionWritesIt(t *testing.T) {
	withX64Regs(t, OpNegate, regInfo{twoAddress: true, inputs: []target.Mask{anyReg}, output: twoCalleeSaved})

	neg := requireOnly(t, requireBuildSSA(t, `fun neg (a int) -> int { return -a; }`), "neg", OpNegate)

	require.Equal(t, LocRegister, neg.Loc.Kind)
	assert.True(t, twoCalleeSaved.Contains(neg.Loc.Reg), "expected r13 or r14, got r%d", neg.Loc.Reg)
}

func TestAllocate_OperandLandsWhereItsReaderReadsIt(t *testing.T) {
	withX64Regs(t, OpNegate, regInfo{twoAddress: true, inputs: []target.Mask{twoCalleeSaved}})

	neg := requireOnly(t, requireBuildSSA(t, `fun neg (a int) -> int { return -a; }`), "neg", OpNegate)

	require.Equal(t, LocRegister, neg.Args[0].Loc.Kind)
	assert.True(t, twoCalleeSaved.Contains(neg.Args[0].Loc.Reg), "expected r13 or r14, got r%d", neg.Args[0].Loc.Reg)
}

func TestAllocate_ReloadLandsWhereItsReaderReadsIt(t *testing.T) {
	withX64Regs(t, OpNegate, regInfo{twoAddress: true, inputs: []target.Mask{twoCalleeSaved}})

	// a is read last, so it's the one that waits out the pressure in a slot and comes back just for the negate
	f := requireFunc(t, requireBuildSSA(t, `
		fun id (x int) -> int { x }
		fun f () -> int {
			let a = id(1); let b = id(2); let c = id(3); let d = id(4); let e = id(5); let g = id(6);
			let h = id(7); let i = id(8); let j = id(9); let k = id(10); let l = id(11); let m = id(12);
			let n = id(13); let o = id(14); let p = id(15); let q = id(16);
			let s = b + c + d + e + g + h + i + j + k + l + m + n + o + p + q;
			-a + s
		}`), "f")

	neg := requireOnly(t, []*Func{f}, "f", OpNegate)
	require.Equal(t, OpStaticLoad, neg.Args[0].Op, "a should be reloaded for the negate")
	require.Equal(t, LocRegister, neg.Args[0].Loc.Kind)
	assert.True(t, twoCalleeSaved.Contains(neg.Args[0].Loc.Reg), "expected r13 or r14, got r%d", neg.Args[0].Loc.Reg)
}

func TestLowerRegs_PinsSingleRegisterOperand(t *testing.T) {
	withX64Regs(t, OpNegate, regInfo{twoAddress: true, inputs: []target.Mask{target.NewMask(target.Reg13)}})

	neg := requireOnly(t, requireBuildSSA(t, `fun neg (a int) -> int { return -a; }`), "neg", OpNegate)

	// the operand is copied into place right before the negate rather than left to the allocator
	assert.Equal(t, OpCopy, neg.Args[0].Op)
	assert.Equal(t, NewReg(target.Reg13), neg.Args[0].Loc)
}

func TestLowerRegs_PinsSingleRegisterResult(t *testing.T) {
	withX64Regs(t, OpNegate, regInfo{twoAddress: true, inputs: []target.Mask{anyReg}, output: target.NewMask(target.Reg12)})

	neg := requireOnly(t, requireBuildSSA(t, `fun neg (a int) -> int { return -a; }`), "neg", OpNegate)

	// the result only holds r12 for as long as it takes to copy it somewhere less contested
	assert.Equal(t, NewReg(target.Reg12), neg.Loc)
	require.Len(t, neg.users, 1)
	assert.Equal(t, OpCopy, neg.users[0].Op)
}

func TestLowerRegs_X64DividePinsDividendAndQuotientToRax(t *testing.T) {
	div := requireOnly(t, requireBuildSSA(t, divideSrc), "div", OpDivide)
	require.Len(t, div.Args, 2)

	// the dividend is copied into rax right before the divide
	assert.Equal(t, OpCopy, div.Args[0].Op)
	assert.Equal(t, NewReg(target.RegA), div.Args[0].Loc)

	// the quotient leaves rax as soon as idiv is done with it
	assert.Equal(t, NewReg(target.RegA), div.Loc)
	require.Len(t, div.users, 1)
	assert.Equal(t, OpCopy, div.users[0].Op)
}

func TestLowerRegs_ARM64DivideIsUnpinned(t *testing.T) {
	f := requireFunc(t, requireBuildSSAFor(t, target.ARM64AAPCS, divideSrc), "div")
	div := requireOnly(t, []*Func{f}, "div", OpDivide)

	// sdiv reads and writes any register, so it reads the parameters' copies without another copy in between
	require.Len(t, div.Args, 2)
	assert.Same(t, requireParamCopy(t, f, 0), div.Args[0])
	assert.Same(t, requireParamCopy(t, f, 1), div.Args[1])

	// and the quotient goes straight into the copy that returns it
	require.Len(t, div.users, 1)
	assert.Equal(t, OpResult, div.users[0].users[0].Op)
}

const divideSrc = `
	fun div (a int, b int) -> int { return a / b; }
	fun main () -> int { return div(7, 2); }
`
