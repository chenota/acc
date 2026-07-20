package codegen

import (
	"slices"
	"strings"
)

// movElim eliminates redundant mov instructions
func movElim(insts []Inst) []Inst {
	return slices.DeleteFunc(insts, isRedundantMove)
}

func isRedundantMove(inst Inst) bool {
	if !strings.HasPrefix(inst.Op, "mov") {
		return false
	}
	return inst.Src1.Kind != KUndefined && inst.Src1 == inst.Dest
}
