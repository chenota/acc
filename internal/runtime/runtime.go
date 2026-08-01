package runtime

import (
	_ "embed"
	"fmt"
	"os"
)

// embed garbage collector runtime's source code in the acc executble
//
//go:embed c/runtime.c
var source string

// WriteSource spills the runtime source to a temporary file
func WriteSource() (string, error) {
	tmpRuntime, err := os.CreateTemp("", "acc_gc_*.c")
	if err != nil {
		return "", err
	}
	defer tmpRuntime.Close()

	if _, err := tmpRuntime.WriteString(source); err != nil {
		os.Remove(tmpRuntime.Name())
		return "", fmt.Errorf("failed to write runtime source: %w", err)
	}

	return tmpRuntime.Name(), nil
}
