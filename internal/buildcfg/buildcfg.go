package buildcfg

import (
	"fmt"
	"runtime"
)

type Arch int

const (
	ArchX64 Arch = iota
	ArchARM64
)

func (a Arch) String() string {
	switch a {
	case ArchX64:
		return "x64"
	case ArchARM64:
		return "arm64"
	default:
		return fmt.Sprintf("Arch(%d)", a)
	}
}

type OS int

const (
	OSLinux OS = iota
	OSDarwin
)

func (o OS) String() string {
	switch o {
	case OSLinux:
		return "linux"
	case OSDarwin:
		return "darwin"
	default:
		return fmt.Sprintf("OS(%d)", o)
	}
}

var (
	targetName string
	targetArch Arch
	targetOS   OS
)

func Target() string { return targetName }

func TargetArch() Arch { return targetArch }

func TargetOS() OS { return targetOS }

func Resolve(target string) error {
	if target == "" {
		target = host()
	}

	switch target {
	case "x64-linux":
		targetArch, targetOS = ArchX64, OSLinux
	case "x64-darwin":
		targetArch, targetOS = ArchX64, OSDarwin
	case "arm64-linux":
		targetArch, targetOS = ArchARM64, OSLinux
	case "arm64-darwin":
		targetArch, targetOS = ArchARM64, OSDarwin
	default:
		return fmt.Errorf("unknown target %q: expected one of x64-linux, x64-darwin, arm64-linux, arm64-darwin", target)
	}

	targetName = target
	return nil
}

func host() string {
	arch := runtime.GOARCH
	// Go calls this one amd64
	if arch == "amd64" {
		arch = "x64"
	}
	return arch + "-" + runtime.GOOS
}
