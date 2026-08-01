package assemble

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chenota/acc/internal/runtime"
)

// Assemble assembles and links a list of x64 instructions into binary using GCC
func Assemble(instructions []string, w io.Writer) error {
	tmpBinary, err := os.CreateTemp("", "acc_bin_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpBinary.Name())
	// we don't want to write to this initially so close for now
	tmpBinary.Close()

	tmpRuntime, err := runtime.WriteSource()
	if err != nil {
		return err
	}
	defer os.Remove(tmpRuntime)

	cmd := assemble(tmpBinary.Name(), tmpRuntime)

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Stdin = bytes.NewBufferString(strings.Join(instructions, "\n") + "\n")

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
