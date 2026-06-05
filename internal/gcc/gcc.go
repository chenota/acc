package gcc

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CompileWithGcc compiles a list of AMD64 instructions into binary
func CompileWithGcc(instructions []string, outputPath string) error {
	// create a temporary file
	tmpFile, err := os.CreateTemp("", "my_compiler_*.s")
	if err != nil {
		return fmt.Errorf("failed to create temp source file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// write the assembly to the temp file
	if _, err := tmpFile.WriteString(strings.Join(instructions, "\n")); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write assembly to temp file: %w", err)
	}
	tmpFile.Close()

	// configure gcc and run
	cmd := exec.Command("gcc", "-g", "-no-pie", tmpFile.Name(), "-o", outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcc failed to assemble source: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}
