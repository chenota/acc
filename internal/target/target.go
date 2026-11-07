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

	Args           []Register
	Results        []Register
	ClosureContext Register

	StackPointer Register
	FramePointer Register
	LinkRegister Register
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
