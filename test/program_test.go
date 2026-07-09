package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/chenota/acc/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgram(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	runWip := runWip()
	asmOnFail := asmOnFail()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(".", entry.Name())

		var isWip bool
		if _, err := os.Stat(filepath.Join(dirPath, "wip")); err == nil {
			isWip = true
		}

		if isWip == runWip {
			t.Run(entry.Name(), func(t *testing.T) {
				mainFile := filepath.Join(dirPath, "main.acc")
				require.FileExists(t, mainFile, "each source directory must contain a main file")

				// on failure dump the assembly acc generated so it's debuggable
				if asmOnFail {
					defer func() {
						if t.Failed() {
							dumpAssembly(t, mainFile)
						}
					}()
				}

				binaryPath := compileProgram(t, mainFile)
				defer os.Remove(binaryPath)

				cmd := exec.Command(binaryPath)

				err := cmd.Run()
				if err != nil {
					var exitErr *exec.ExitError
					require.ErrorAs(t, err, &exitErr, "unexpected runtime error", err.Error())
				}

				actualStatus := cmd.ProcessState.ExitCode()
				// translate -1 exit code to shell's 128 + signal convention
				if actualStatus == -1 {
					if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
						actualStatus = 128 + int(ws.Signal())
					}
				}
				verifyGoldenStatus(t, dirPath, actualStatus)
			})
		}
	}
}

func compileProgram(t *testing.T, mainFile string) string {
	t.Helper()

	tmpBinary, err := os.CreateTemp("", "acc_*")
	require.NoError(t, err)

	// immediately close our temporary file to avoid conflicts
	tmpBinary.Close()

	err = os.Chmod(tmpBinary.Name(), 0755)
	require.NoError(t, err)

	root := cmd.NewRootCommand()
	root.SetArgs([]string{
		mainFile,
		"-o", tmpBinary.Name(),
	})

	require.NoError(t, root.Execute(), "failed to compile program")

	return tmpBinary.Name()
}

func verifyGoldenStatus(t *testing.T, dirPath string, actualStatus int) {
	t.Helper()

	statusPath := filepath.Join(dirPath, "status.golden")

	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		require.NoError(t, err, "failed to read status.golden file")
	}

	statusStr := strings.TrimSpace(string(statusBytes))

	expectedStatus, err := strconv.Atoi(statusStr)
	require.NoError(t, err)

	assert.Equal(t, expectedStatus, actualStatus, "actual status does not match golden status")
}

func asmOnFail() bool {
	v := os.Getenv("ASM_ON_FAIL")
	return v == "1" || v == "true"
}

func runWip() bool {
	v := os.Getenv("RUN_WIP")
	return v == "1" || v == "true"
}

// dumpAssembly makes a best-effort attempt to log the assembly acc generates for mainFile with the -S flag.
// if it can't just print out the error preventing assembly generation.
func dumpAssembly(t *testing.T, mainFile string) {
	t.Helper()

	tmpAsm, err := os.CreateTemp("", "acc_*.s")
	if err != nil {
		t.Logf("could not create temp file for assembly: %v", err)
		return
	}
	tmpAsm.Close()
	defer os.Remove(tmpAsm.Name())

	root := cmd.NewRootCommand()
	root.SetArgs([]string{
		mainFile,
		"-S",
		"-o", tmpAsm.Name(),
	})
	if err := root.Execute(); err != nil {
		t.Logf("could not generate assembly for %s: %v", mainFile, err)
		return
	}

	asmBytes, err := os.ReadFile(tmpAsm.Name())
	if err != nil {
		t.Logf("could not read generated assembly for %s: %v", mainFile, err)
		return
	}

	t.Logf("generated assembly for %s:\n%s", mainFile, string(asmBytes))
}
