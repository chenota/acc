package codegen

import (
	"fmt"

	"github.com/chenota/acc/internal/buildcfg"
	"github.com/chenota/acc/internal/ssa"
)

type symbols struct {
	os buildcfg.OS
}

func (s symbols) symbolPrefix() string {
	switch s.os {
	case buildcfg.OSDarwin:
		// The Mach-O ABI underscore-prefixes C symbols.
		return "_"
	default:
		// ELF C symbols have no prefix
		return ""
	}
}

func (s symbols) localPrefix() string {
	switch s.os {
	case buildcfg.OSDarwin:
		// Mach-O keeps L labels out of the symbol table
		return "L"
	default:
		// ELF keeps .L labels out of the symbol table
		return ".L"
	}
}

// funcLabel is the symbol a function is defined and called under
func (s symbols) funcLabel(f *ssa.Func) string {
	return s.symbolPrefix() + f.Name()
}

// blockLabel names a block, scoped to its function since block ids restart in each one
func (s symbols) blockLabel(f *ssa.Func, b *ssa.Block) string {
	return fmt.Sprintf("%s%s.%d", s.localPrefix(), f.Name(), b.Id)
}

func (s symbols) epilogueLabel(f *ssa.Func) string {
	return fmt.Sprintf("%s%s.ret", s.localPrefix(), f.Name())
}
