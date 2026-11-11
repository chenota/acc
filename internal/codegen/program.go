package codegen

import (
	"fmt"
	"strings"
)

// Program is a target's machine code, one function at a time.
type Program[I fmt.Stringer] struct {
	Funcs []*Func[I]
}

// Func is a function symbol and the blocks of code laid out under it, in order.
type Func[I fmt.Stringer] struct {
	Name   string
	Global bool
	Blocks []*Block[I]
}

// Block is a run of instructions, entered at the top through its label or by falling out of the block before it.
type Block[I fmt.Stringer] struct {
	Label string // empty when nothing jumps here
	Insts []I
}

// String renders the program as assembly text in the target's syntax.
func (p *Program[I]) String() string {
	var b strings.Builder
	b.WriteString("\t.text\n")
	for _, f := range p.Funcs {
		if f.Global {
			b.WriteString("\t.globl " + f.Name + "\n")
		}
		b.WriteString(f.Name + ":\n")
		for _, blk := range f.Blocks {
			if blk.Label != "" {
				b.WriteString(blk.Label + ":\n")
			}
			for _, inst := range blk.Insts {
				b.WriteString("\t" + inst.String() + "\n")
			}
		}
	}
	return b.String()
}
