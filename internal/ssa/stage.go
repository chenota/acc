package ssa

// a stageHook runs around one stage of the pipeline
type stageHook func(f *Func, stage string, next func() error) error

// stageHooks wrap every stage, with the first one outermost.
var stageHooks []stageHook

// runStage runs a single stage with hooks
func runStage(f *Func, name string, run func(*Func) error) error {
	next := func() error { return run(f) }
	for _, h := range stageHooks {
		inner := next
		next = func() error { return h(f, name, inner) }
	}

	return next()
}
