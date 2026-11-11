package assemble

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chenota/acc/internal/runtime"
)

// Assemble assembles and links assembly source into a binary using the host's C toolchain
func Assemble(src string, w io.Writer) error {
	tmpBinary, err := os.CreateTemp("", "acc_bin_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpBinary.Name())
	// we don't want to write to this initially so close for now
	tmpBinary.Close()

	tmpRuntime, err := runtime.WriteObject()
	if err != nil {
		return err
	}
	defer os.Remove(tmpRuntime)

	cmd := assemble(tmpBinary.Name(), tmpRuntime)

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Stdin = strings.NewReader(src)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed to assemble source: %w (stderr: %s)", cmd.Args[0], err, stderr.String())
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
