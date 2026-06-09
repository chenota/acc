package gcc

type gccOptions struct {
	isStatic bool
}

type Option func(*gccOptions)

func WithStatic() Option {
	return func(o *gccOptions) {
		o.isStatic = true
	}
}
