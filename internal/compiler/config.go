package compiler

type compilerOptions struct {
	isAssembly bool
	target     string
}

type Option func(*compilerOptions)

// WithAssemblyOnly tells acc to emit text assembly.
func WithAssemblyOnly() Option {
	return func(o *compilerOptions) {
		o.isAssembly = true
	}
}

// WithTarget tells acc which target to compile for.
func WithTarget(target string) Option {
	return func(o *compilerOptions) {
		o.target = target
	}
}
