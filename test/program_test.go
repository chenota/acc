package test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenota/acc/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfig is the contents of a test directory's test.json file.
type testConfig struct {
	Name   string   `json:"name"`
	Tags   []string `json:"tags"`
	Status *int     `json:"status"`
}

func TestProgram(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	userTag, userNegative := tag(t)
	verboseFail := verboseFail()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(".", entry.Name())
		config := readTestConfig(t, dirPath)

		t.Run(config.Name, func(t *testing.T) {
			if userTag == "" || !userNegative == slices.Contains(config.Tags, userTag) {
				mainFile := filepath.Join(dirPath, "main.acc")
				require.FileExists(t, mainFile, "each source directory must contain a main file")

				// on failure dump the assembly acc generated so it's debuggable
				if verboseFail {
					defer func() {
						if t.Failed() {
							dumpAssembly(t, mainFile)
						}
					}()
				}

				binaryPath := compileProgram(t, mainFile)
				defer os.Remove(binaryPath)

				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()

				cmd := exec.CommandContext(ctx, binaryPath)
				err := cmd.Run()

				// make sure context did not time out
				require.NotErrorIs(t, ctx.Err(), context.DeadlineExceeded, "test timeout")

				if err != nil {
					var exitErr *exec.ExitError
					require.ErrorAs(t, err, &exitErr, "unexpected runtime error", err.Error())
				}

				actualStatus := cmd.ProcessState.ExitCode()
				verifyStatus(t, config, actualStatus)
			}
		})
	}
}

// readTestConfig reads and parses the test.json file in dirPath.
func readTestConfig(t *testing.T, dirPath string) testConfig {
	t.Helper()

	configBytes, err := os.ReadFile(filepath.Join(dirPath, "test.json"))
	require.NoError(t, err, "each test directory must contain a test.json file")

	var config testConfig
	require.NoError(t, json.Unmarshal(configBytes, &config), "failed to parse test.json")
	require.NotEmpty(t, config.Name, "test.json must contain a name field")

	for _, testTag := range config.Tags {
		require.NotContains(t, testTag, "~", "illegal character in test tags: ~")
	}

	return config
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

func verifyStatus(t *testing.T, config testConfig, actualStatus int) {
	t.Helper()

	// skip this check if the test does not specify an expected status
	if config.Status == nil {
		return
	}

	assert.Equal(t, *config.Status, actualStatus, "actual status does not match expected status")
}

func verboseFail() bool {
	v := os.Getenv("VERBOSE_FAIL")
	return v == "1" || v == "true"
}

// tag returns a user-supplied tag and a bool indicating if it is a ~ operation
func tag(t *testing.T) (value string, isNegative bool) {
	t.Helper()

	value = os.Getenv("TAG")
	if strings.HasPrefix(value, "~") {
		isNegative = true
		value = value[1:]

		require.NotContains(t, value, "~", "illegal character in user-supplied tag: ~")
	}

	return
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
