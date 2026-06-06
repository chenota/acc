package lexer

type Option func(*config)

type config struct {
	FileName string
}

func WithFileName(name string) Option {
	return func(c *config) {
		c.FileName = name
	}
}
