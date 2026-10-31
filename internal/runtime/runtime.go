package runtime

import (
	"fmt"
	"os"
)

// precompile the garbage collector runtime into an x64 object for each supported OS
//
//go:generate clang -c -fPIE -target x86_64-linux-gnu c/runtime.c -o c/runtime_linux.o
//go:generate clang -c -target x86_64-apple-macos11 c/runtime.c -o c/runtime_darwin.o

// WriteObject spills the precompiled runtime object to a temporary file
func WriteObject() (string, error) {
	tmpRuntime, err := os.CreateTemp("", "acc_gc_*.o")
	if err != nil {
		return "", err
	}
	defer tmpRuntime.Close()

	if _, err := tmpRuntime.Write(object); err != nil {
		os.Remove(tmpRuntime.Name())
		return "", fmt.Errorf("failed to write runtime object: %w", err)
	}

	return tmpRuntime.Name(), nil
}
