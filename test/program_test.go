package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/chenota/acc/internal/buildcfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfig is the contents of a test directory's test.json file.
type testConfig struct {
	Name   string        `json:"name"`
	Tags   []string      `json:"tags"`
	Status oneOf[int]    `json:"status"`
	Signal oneOf[string] `json:"signal"`
}

// oneOf is every value a test accepts, which test.json gives as either a lone value or a list of them
type oneOf[T any] []T

func (o *oneOf[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	// a lone value is a list of just that one
	if data[0] != '[' {
		data = slices.Concat([]byte("["), data, []byte("]"))
	}

	var vs []T
	if err := json.Unmarshal(data, &vs); err != nil {
		return err
	}
	if len(vs) == 0 {
		return errors.New("expected at least one value, not an empty list")
	}
	*o = vs
	return nil
}

// signals are the ones a test may expect a program to die by
var signals = map[string]syscall.Signal{
	"SIGABRT": syscall.SIGABRT,
	"SIGFPE":  syscall.SIGFPE,
	"SIGILL":  syscall.SIGILL,
	"SIGSEGV": syscall.SIGSEGV,
	"SIGTRAP": syscall.SIGTRAP,
}

func TestProgram(t *testing.T) {
	// compile the acc executable with go.
	accPath := filepath.Join(t.TempDir(), "acc")
	out, err := exec.CommandContext(t.Context(), "go", "build", "-tags", "ssacheck", "-o", accPath, "github.com/chenota/acc").CombinedOutput()
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
				state := runProgram(t, binaryPath)
				verifyStatus(t, config, state.ExitCode())
				verifySignal(t, config, state)
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

	for _, s := range config.Signal {
		require.Contains(t, signals, s, "unknown signal in test.json")
	}

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

	out, err := exec.CommandContext(ctx, accPath, "--target", target(), "-o", binaryPath, mainFile).CombinedOutput()
	require.NoError(t, err, "failed to compile program:\n%s", out)

	return binaryPath
}

const (
	compileTimeout = 10 * time.Second
	runTimeout     = 3 * time.Second
	reapGrace      = 1 * time.Second
)

// runProgram runs the binary at binaryPath and returns how it ended.
func runProgram(t *testing.T, binaryPath string) *os.ProcessState {
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

	return cmd.ProcessState
}

func verifyStatus(t *testing.T, config testConfig, actualStatus int) {
	t.Helper()

	// skip this check if the test does not specify an expected status
	if config.Status == nil {
		return
	}

	assert.Contains(t, config.Status, actualStatus, "actual status is not one the test expects")
}

func verifySignal(t *testing.T, config testConfig, state *os.ProcessState) {
	t.Helper()

	// skip this check if the test does not expect the program to die by a signal
	if config.Signal == nil {
		return
	}
	expected := strings.Join(config.Signal, " or ")

	status, ok := state.Sys().(syscall.WaitStatus)
	require.True(t, ok, "no wait status to read a signal from")
	require.True(t, status.Signaled(), "program exited with status %d instead of dying by %s", state.ExitCode(), expected)

	actual := status.Signal()
	died := slices.ContainsFunc(config.Signal, func(name string) bool { return signals[name] == actual })
	assert.True(t, died, "program died by %q instead of %s", actual, expected)
}

// target returns the target to compile test programs for, which is the host's unless TARGET names another
func target() string {
	if v := os.Getenv("TARGET"); v != "" {
		return v
	}
	return buildcfg.Host()
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

	if out, err := exec.CommandContext(ctx, accPath, "--target", target(), "-S", "-o", tmpAsm.Name(), mainFile).CombinedOutput(); err != nil {
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
