package assemble

import "os/exec"

func assemble(bin string, runtime string) *exec.Cmd {
	args := []string{
		"-x", "assembler",
		"-",
		"-x", "none",
		runtime,
		"-arch", "x86_64",
		"-o", bin,
	}

	return exec.Command("clang", args...)
}
