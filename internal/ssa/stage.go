//go:build !ssacheck

package ssa

// runStage runs the stage called name over f. Really just used for compatibility with the checker.
func runStage(f *Func, _ string, run func(*Func) error) error {
	return run(f)
}
