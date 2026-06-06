package parser

import (
	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/lexer"
)

type parser struct {
	err *diagnostic.Error
	t   *lexer.TokenList
}

func (p *parser) markErr(message string) {
	if p.err == nil || p.t.Pos().GreaterThan(p.err.Pos()) {
		p.err = diagnostic.NewError(message, p.t.Pos())
	}
}
