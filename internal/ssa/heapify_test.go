package ssa

import (
	"slices"
	"testing"

	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeapify_RewritesAnEscapingLocal(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun f () -> *int { let x = 1; return &x; }
		fun main () -> int { let p = f(); return *p; }`)
	f := requireFunc(t, funcs, "f")

	alloc := requireAllocate(t, f)

	// the returned address is now the allocation itself
	assert.Equal(t, alloc, f.Entry.Control)
	assert.True(t, types.Equal(types.Pointer(types.Int()), alloc.Type),
		"expected *int, got %v", alloc.Type)

	// nothing still takes the address of a frame slot, and the slot is gone
	assert.Empty(t, findValues(slices.Collect(f.UnorderedValues()), OpLocalAddr))
	assert.Nil(t, slotNamed(f, "x"))

	// allocating is a call, so regalloc has to treat it as one
	assert.Equal(t, register.CallerSaved, alloc.Clobbers())
	assert.True(t, alloc.NeedsRegister(), "the allocation produces a pointer")
}

func TestHeapify_NestedPointers(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun deep () -> **int {
			let x = 1;
			let p = &x;
			p = &x;
			let q = &p;
			return q;
		}
		fun main () -> int { let r = deep(); return **r; }`)
	f := requireFunc(t, funcs, "deep")

	// both x and p are reachable from the caller, q only holds the address
	assert.Len(t, findValues(slices.Collect(f.UnorderedValues()), OpAllocate), 2)
	assert.Nil(t, slotNamed(f, "x"))
	assert.Nil(t, slotNamed(f, "p"))

	for _, f := range funcs {
		for v := range f.UnorderedValues() {
			if slot := v.Slot(); slot != nil {
				assert.Contains(t, f.Slots, slot, "%v names a slot that is no longer in the frame", v.Op)
			}
		}
	}
}

func TestHeapify_LeavesNonEscapingSlotsAlone(t *testing.T) {
	// the address is taken and spent before the function returns
	f := requireFunc(t, requireBuildSSA(t, `
		fun main () -> int {
			let x = 1;
			let p = &x;
			return *p;
		}`), "main")

	assert.Empty(t, findValues(slices.Collect(f.UnorderedValues()), OpAllocate))
	assert.NotNil(t, slotNamed(f, "x"))
	assert.NotEmpty(t, findValues(slices.Collect(f.UnorderedValues()), OpLocalAddr))
}

// requireAllocate returns the single OpAllocate in f.
func requireAllocate(t *testing.T, f *Func) *Value {
	t.Helper()
	allocs := findValues(slices.Collect(f.UnorderedValues()), OpAllocate)
	require.Len(t, allocs, 1)
	return allocs[0]
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
