package codegen

import "github.com/chenota/acc/internal/buildcfg"

func symbolPrefix() string {
	switch buildcfg.TargetOS {
	case buildcfg.OSDarwin:
		// The Mach-O ABI underscore-prefixes C symbols.
		return "_"
	default:
		// ELF C symbols have no prefix
		return ""
	}
}

func localPrefix() string {
	switch buildcfg.TargetOS {
	case buildcfg.OSDarwin:
		// Mach-O keeps L labels out of the symbol table
		return "L"
	default:
		// ELF keeps .L labels out of the symbol table
		return ".L"
	}
}
