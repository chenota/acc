package assemble

import (
	"os/exec"
	"runtime"

	"github.com/chenota/acc/internal/buildcfg"
)

func assemble(bin string, runtime string) *exec.Cmd {
	args := []string{
		"-x", "assembler",
		"-",
		"-x", "none",
		runtime,
	}

	switch buildcfg.TargetOS() {
	case buildcfg.OSDarwin:
		args = append(args, "-arch", clangArch(), "-o", bin)
		return exec.Command("clang", args...)
	default:
		args = append(args, "-z", "noexecstack", "-o", bin)
		return exec.Command(gcc(), args...)
	}
}

func clangArch() string {
	switch buildcfg.TargetArch() {
	case buildcfg.ArchARM64:
		return "arm64"
	default:
		return "x86_64"
	}
}

func gcc() string {
	if hostArch() == buildcfg.TargetArch() {
		return "gcc"
	}
	switch buildcfg.TargetArch() {
	case buildcfg.ArchARM64:
		return "aarch64-linux-gnu-gcc"
	default:
		return "x86_64-linux-gnu-gcc"
	}
}

func hostArch() buildcfg.Arch {
	if runtime.GOARCH == "arm64" {
		return buildcfg.ArchARM64
	}
	return buildcfg.ArchX64
}
