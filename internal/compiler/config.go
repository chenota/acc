package compiler

type compilerOptions struct {
	isAssembly bool
	isStatic   bool
}

type Option func(*compilerOptions)

// WithAssemblyOnly tells acc to emit text assembly.
func WithAssemblyOnly() Option {
	return func(o *compilerOptions) {
		o.isAssembly = true
	}
}

func WithStaticCompilation() Option {
	return func(o *compilerOptions) {
		o.isStatic = true
	}
}
