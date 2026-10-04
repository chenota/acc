package ssa

import "github.com/chenota/acc/internal/types"

// Alloc is the garbage collector's allocator.
var Alloc = newExternFunc("acc_alloc",
	types.Function([]*types.Type{types.Int()}, types.Pointer(types.Unit())),
	[]paramSummary{{heap: nil, results: []*int{nil}}})
