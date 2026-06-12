//go:build !darwin

package assemble

import "os/exec"

func assembleCmd(bin string) *exec.Cmd {
	args := []string{
		"-x", "assembler",
		"-",
		"-no-pie",
		"-nostdlib",
		"-o", bin,
	}

	return exec.Command("gcc", args...)
}
