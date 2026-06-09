package gcc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CompileWithGcc compiles a list of AMD64 instructions into binary using GCC
func CompileWithGcc(instructions []string, w io.Writer, opts ...Option) error {
	var config gccOptions
	for _, o := range opts {
		o(&config)
	}

	tmpBinary, err := os.CreateTemp("", "acc_bin_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpBinary.Name())
	// we don't want to write to this initially so close for now
	tmpBinary.Close()

	args := []string{"-x", "assembler", "-", "-no-pie", "-o", tmpBinary.Name()}

	if config.isStatic {
		args = append(args, "-static")
	}

	cmd := exec.Command("gcc", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	cmd.Stdout = w

	cmd.Stdin = bytes.NewBufferString(strings.Join(instructions, "\n"))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcc failed to assemble source: %w (stderr: %s)", err, stderr.String())
	}

	// re-open the temporary file now that GCC has written to it
	binaryReader, err := os.Open(tmpBinary.Name())
	if err != nil {
		return fmt.Errorf("failed to open linked binary for reading: %w", err)
	}
	defer binaryReader.Close()

	_, err = io.Copy(w, binaryReader)
	if err != nil {
		return fmt.Errorf("failed to copy linked binary bytes to writer: %w", err)
	}

	return nil
}
