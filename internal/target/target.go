package target

import (
	"fmt"

	"github.com/chenota/acc/internal/buildcfg"
)

// Target describes a register file and calling conventions
type Target struct {
	Allocatable Mask
	CallerSaved Mask
	CalleeSaved Mask

	Args           []Register
	Results        []Register
	ClosureContext Register

	StackPointer Register
	FramePointer Register
}

func For(arch buildcfg.Arch, os buildcfg.OS) (*Target, error) {
	switch arch {
	case buildcfg.ArchX64:
		// Linux and macOS share System V
		return X64SysV, nil
	default:
		return nil, fmt.Errorf("no register target for %s yet", arch)
	}
}
