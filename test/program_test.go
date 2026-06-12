package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chenota/acc/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgram(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	wipEnv := os.Getenv("ACC_RUN_WIP")
	runWip := (wipEnv == "1" || wipEnv == "true")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(".", entry.Name())

		var isWip bool
		if _, err := os.Stat(filepath.Join(dirPath, "wip")); err == nil {
			isWip = true
		}

		if runWip || !isWip {
			t.Run(entry.Name(), func(t *testing.T) {
				mainFile := filepath.Join(dirPath, "main.acc")
				require.FileExists(t, mainFile, "each source directory must contain a main file")

				binaryPath := compileProgram(t, mainFile)
				defer os.Remove(binaryPath)

				cmd := exec.Command(binaryPath)

				err := cmd.Run()
				if err != nil {
					var exitErr *exec.ExitError
					require.ErrorAs(t, err, &exitErr, "unexpected runtime error", err.Error())
				}

				actualStatus := cmd.ProcessState.ExitCode()
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

	// for now we're only doing happy path tests so don't need to worry about compilation failures
	require.NoError(t, root.Execute())

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
