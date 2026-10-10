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
	// compile the acc executable with go.
	accPath := filepath.Join(t.TempDir(), "acc")
	out, err := exec.CommandContext(t.Context(), "go", "build", "-o", accPath, "github.com/chenota/acc").CombinedOutput()
	require.NoError(t, err, "failed to build acc:\n%s", out)

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
			t.Parallel()

			if userTag == "" || !userNegative == slices.Contains(config.Tags, userTag) {
				mainFile := filepath.Join(dirPath, "main.acc")
				require.FileExists(t, mainFile, "each source directory must contain a main file")

				// on failure dump the assembly acc generated so it's debuggable
				defer func() {
					if t.Failed() && verboseFail {
						dumpAssembly(t, accPath, mainFile)
					}
				}()

				binaryPath := compileProgram(t, accPath, mainFile)
				actualStatus := runProgram(t, binaryPath)
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

func compileProgram(t *testing.T, accPath, mainFile string) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "main.out")

	ctx, cancel := context.WithTimeout(t.Context(), compileTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, accPath, "-o", binaryPath, mainFile).CombinedOutput()
	require.NoError(t, err, "failed to compile program:\n%s", out)

	return binaryPath
}

const (
	compileTimeout = 10 * time.Second
	runTimeout     = 3 * time.Second
	reapGrace      = 1 * time.Second
)

// runProgram runs the binary at binaryPath and returns its exit status.
func runProgram(t *testing.T, binaryPath string) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath)
	require.NoError(t, cmd.Start(), "failed to start program")

	// wait off to the side so a process that can't be reaped doesn't hang the whole suite
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		select {
		case <-done:
			require.FailNow(t, "test timeout")
		case <-time.After(reapGrace):
			require.FailNow(t, "test timeout", "program could not be reaped after being killed")
		}
	}

	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "unexpected runtime error", err.Error())
	}

	return cmd.ProcessState.ExitCode()
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

// dumpAssembly makes a best-effort attempt to log the assembly acc generates for mainFile
func dumpAssembly(t *testing.T, accPath, mainFile string) {
	t.Helper()

	tmpAsm, err := os.CreateTemp("", "acc_*.s")
	if err != nil {
		t.Logf("could not create temp file for assembly: %v", err)
		return
	}
	tmpAsm.Close()
	defer os.Remove(tmpAsm.Name())

	ctx, cancel := context.WithTimeout(t.Context(), compileTimeout)
	defer cancel()

	if out, err := exec.CommandContext(ctx, accPath, "-S", "-o", tmpAsm.Name(), mainFile).CombinedOutput(); err != nil {
		t.Logf("could not generate assembly for %s: %v\n%s", mainFile, err, out)
		return
	}

	asmBytes, err := os.ReadFile(tmpAsm.Name())
	if err != nil {
		t.Logf("could not read generated assembly for %s: %v", mainFile, err)
		return
	}

	t.Logf("generated assembly for %s:\n%s", mainFile, string(asmBytes))
}
