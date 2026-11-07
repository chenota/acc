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

type OS int

const (
	OSLinux OS = iota
	OSDarwin
)

var (
	Target     string
	TargetArch Arch
	TargetOS   OS
)

func Resolve(target string) error {
	if target == "" {
		target = host()
	}

	switch target {
	case "x64-linux":
		TargetArch, TargetOS = ArchX64, OSLinux
	case "x64-darwin":
		TargetArch, TargetOS = ArchX64, OSDarwin
	case "arm64-linux":
		TargetArch, TargetOS = ArchARM64, OSLinux
	case "arm64-darwin":
		TargetArch, TargetOS = ArchARM64, OSDarwin
	default:
		return fmt.Errorf("unknown target %q: expected one of x64-linux, x64-darwin, arm64-linux, arm64-darwin", target)
	}

	Target = target
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
