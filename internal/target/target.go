package target

import (
	"fmt"

	"github.com/chenota/acc/internal/buildcfg"
)

// Target describes a register file and calling conventions
type Target struct {
	Arch buildcfg.Arch

	Allocatable Mask
	CallerSaved Mask
	CalleeSaved Mask
	// Registers the allocator never hands out, which codegen may clobber within an instruction sequence but never
	// rely on across a call
	Scratch []Register

	Args           []Register
	Results        []Register
	ClosureContext Register

	StackPointer Register
	FramePointer Register
	LinkRegister Register

	// How far above the frame pointer the caller's outgoing records start
	FrameRecordSize int
	// The fewest bytes a stack argument takes up and aligns to; 1 packs them at their natural size and alignment
	StackArgSlotSize int
}

func For(arch buildcfg.Arch, os buildcfg.OS) (*Target, error) {
	switch arch {
	case buildcfg.ArchX64:
		// Linux and macOS share System V
		return X64SysV, nil
	case buildcfg.ArchARM64:
		if os == buildcfg.OSDarwin {
			return ARM64Apple, nil
		}
		return ARM64AAPCS, nil
	default:
		return nil, fmt.Errorf("no register target for %s", arch)
	}
}
