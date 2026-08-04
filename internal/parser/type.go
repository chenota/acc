package parser

import (
	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func (p *parser) parseType() (*ir.Node, bool) {
	loc := p.t.Mark()
	pos := p.t.Pos()

	t, err := p.typ()
	if err != nil {
		p.t.Restore(loc)
		p.markErrValue(err)
		return nil, false
	}

	return &ir.Node{
		Op:   ir.OpType,
		Pos:  pos,
		Type: t,
	}, true
}

func (p *parser) typ() (*types.Type, error) {
	tok, ok := p.t.Peek()
	if !ok {
		return nil, diagnostic.NewError(p.t.Pos(), "expected type")
	}

	switch tok.Kind {
	case lexer.KStar:
		p.t.Advance()

		subType, err := p.typ()
		if err != nil {
			return nil, err
		}

		return types.Pointer(subType), nil
	case lexer.KFunKw:
		return p.functionType()
	case lexer.KLParen:
		p.t.Advance()

		if _, ok := p.t.Expect(lexer.KRParen); !ok {
			return nil, diagnostic.NewError(p.t.Pos(), "expected closing parenthesis in unit type")
		}

		return types.Unit(), nil
	case lexer.KIntKw:
		p.t.Advance()

		return types.Int(), nil
	default:
		return nil, diagnostic.NewError(tok.Pos, "expected type")
	}
}

func (p *parser) functionType() (*types.Type, error) {
	if _, ok := p.t.Expect(lexer.KFunKw); !ok {
		return nil, diagnostic.NewError(p.t.Pos(), "expected fun keyword in function type")
	}

	if _, ok := p.t.Expect(lexer.KLParen); !ok {
		return nil, diagnostic.NewError(p.t.Pos(), "expected opening parenthesis in function type")
	}

	var params []*types.Type
	if _, ok := p.t.Expect(lexer.KRParen); !ok {
		var err error
		params, err = p.typeList()
		if err != nil {
			return nil, err
		}

		if _, ok := p.t.Expect(lexer.KRParen); !ok {
			return nil, diagnostic.NewError(p.t.Pos(), "missing closing parenthesis in function type")
		}
	}

	if _, ok := p.t.Expect(lexer.KArrow); !ok {
		return types.Function(params, types.Unit()), nil
	}

	result, err := p.typ()
	if err != nil {
		return nil, err
	}

	return types.Function(params, result), nil
}

func (p *parser) typeList() ([]*types.Type, error) {
	// parse head
	head, err := p.typ()
	if err != nil {
		return nil, err
	}
	l := []*types.Type{head}

	// parse rest of list in a loop
	for {
		if _, ok := p.t.Expect(lexer.KComma); !ok {
			return l, nil
		}

		v, err := p.typ()
		if err != nil {
			return nil, err
		}

		l = append(l, v)
	}
}
