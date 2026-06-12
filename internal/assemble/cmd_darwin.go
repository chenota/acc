package assemble

import "os/exec"

func assembleCmd(bin string) *exec.Cmd {
	args := []string{
		"-x", "assembler",
		"-",
		"-arch", "x86_64",
		"-o", bin,
	}

	return exec.Command("clang", args...)
}
