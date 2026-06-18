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
	if p.t.Pos().GreaterThan(p.err.Pos()) {
		p.err = diagnostic.NewError(message, p.t.Pos())
	}
}

func (p *parser) markErrDiagnostic(e *diagnostic.Error) {
	if e.Pos().GreaterThan(p.err.Pos()) {
		p.err = e
	}
}
