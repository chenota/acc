package runtime

import (
	"embed"
	"fmt"
	"os"

	"github.com/chenota/acc/internal/buildcfg"
)

// precompile the garbage collector runtime into an object for each target
//
//go:generate clang -c -fPIE -target x86_64-linux-gnu c/runtime.c -o c/runtime_x64-linux.o
//go:generate clang -c -target x86_64-apple-macos11 c/runtime.c -o c/runtime_x64-darwin.o
//go:generate clang -c -fPIE -target aarch64-linux-gnu c/runtime.c -o c/runtime_arm64-linux.o
//go:generate clang -c -target arm64-apple-macos11 c/runtime.c -o c/runtime_arm64-darwin.o

//go:embed c/*.o
var objects embed.FS

// WriteObject spills the target's precompiled runtime object to a temporary file
func WriteObject() (string, error) {
	object, err := objects.ReadFile(fmt.Sprintf("c/runtime_%s.o", buildcfg.Target()))
	if err != nil {
		return "", fmt.Errorf("no runtime object for target %s: %w", buildcfg.Target(), err)
	}

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
