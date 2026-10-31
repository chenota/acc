package assemble

import "os/exec"

func assemble(bin string, runtime string) *exec.Cmd {
	args := []string{
		"-x", "assembler",
		"-",
		"-x", "none",
		runtime,
		"-z", "noexecstack",
		"-o", bin,
	}

	return exec.Command("gcc", args...)
}
