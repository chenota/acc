package parser

import (
	"errors"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/lexer"
)

type parser struct {
	err *diagnostic.Error
	t   *lexer.TokenList
}

func (p *parser) markErr(message string) {
	if p.t.Pos().GreaterThan(p.err.Pos()) {
		p.err = diagnostic.NewError(p.t.Pos(), "%s", message)
	}
}

func (p *parser) markErrValue(err error) {
	if diagnosticErr, ok := errors.AsType[*diagnostic.Error](err); ok {
		p.markErrDiagnostic(diagnosticErr)
	} else {
		p.markErr(err.Error())
	}
}

func (p *parser) markErrDiagnostic(e *diagnostic.Error) {
	if e.Pos().GreaterThan(p.err.Pos()) {
		p.err = e
	}
}
