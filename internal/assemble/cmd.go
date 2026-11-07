package assemble

import (
	"os/exec"

	"github.com/chenota/acc/internal/buildcfg"
)

func assemble(bin string, runtime string) *exec.Cmd {
	args := []string{
		"-x", "assembler",
		"-",
		"-x", "none",
		runtime,
	}

	switch buildcfg.TargetOS {
	case buildcfg.OSDarwin:
		args = append(args, "-arch", clangArch(), "-o", bin)
		return exec.Command("clang", args...)
	default:
		args = append(args, "-z", "noexecstack", "-o", bin)
		return exec.Command("gcc", args...)
	}
}

func clangArch() string {
	switch buildcfg.TargetArch {
	case buildcfg.ArchARM64:
		return "arm64"
	default:
		return "x86_64"
	}
}
