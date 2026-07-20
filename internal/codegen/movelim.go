package codegen

import (
	"slices"
	"strings"
)

// movElim eliminates redundant mov instructions
func movElim(insts []Inst) []Inst {
	return slices.DeleteFunc(insts, func(v Inst) bool {
		return strings.HasPrefix(v.Op, "mov") && v.Src1 == v.Dest
	})
}
