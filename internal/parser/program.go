package parser

import (
	"errors"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
)

func ParseProgram(t *lexer.TokenList) ([]*ir.Node, error) {
	var globalStmts []*ir.Node

	p := &parser{
		t: t,
	}

	// consume statements until we can't
	for {
		if s, ok := p.parseStmt(); ok {
			globalStmts = append(globalStmts, s)
		} else {
			break
		}
	}

	// if not enough tokens were consumed then there was a parsing error
	if !t.Empty() {
		if p.err == nil {
			return nil, errors.New("unknown parsing error: could not parse entire file")
		}
		return nil, p.err
	}

	// vertial slice check: for now we should have a single function called main
	if len(globalStmts) != 1 || globalStmts[0].Op != ir.OpFunction {
		return nil, errors.New("vertical slice error: should have a single function called 'main'")
	}

	return globalStmts, nil
}

func (p *parser) parseBlock() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Op:  ir.OpBlock,
		Pos: p.t.Pos(),
	}

	_, ok := p.t.Expect(lexer.KLBracket)
	if !ok {
		p.markErr("expected opening bracket '{'")
		p.t.Restore(loc)
		return nil, false
	}

	var stmts []*ir.Node
	for {
		s, ok := p.parseStmt()
		if !ok {
			break
		}
		s.Parent = n
		stmts = append(stmts, s)
	}
	n.List = stmts

	_, ok = p.t.Expect(lexer.KRBracket)
	if !ok {
		p.markErr("expected closing bracket '}' to match opening bracket")
		p.t.Restore(loc)
		return nil, false
	}

	return n, true
}

func (p *parser) parseStmt() (*ir.Node, bool) {
	if f, ok := p.parseFunction(); ok {
		return f, true
	}

	return p.parseReturn()
}

func (p *parser) parseReturn() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Op:  ir.OpReturn,
		Pos: p.t.Pos(),
	}

	if _, ok := p.t.Expect(lexer.KReturnKw); !ok {
		p.markErr("expected return keyword")
		p.t.Restore(loc)
		return nil, false
	}

	e, ok := p.parseExpr()
	if !ok {
		p.t.Restore(loc)
		return nil, false
	}
	e.Parent = n
	n.List = []*ir.Node{e}

	if _, ok = p.t.Expect(lexer.KSemicolon); !ok {
		p.markErr("expected semicolon")
		p.t.Restore(loc)
		return nil, false
	}

	return n, true
}

func (p *parser) parseFunction() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Op:        ir.OpFunction,
		Pos:       p.t.Pos(),
		Signature: &ir.Signature{},
	}

	_, ok := p.t.Expect(lexer.KFunKw)
	if !ok {
		p.markErr("expected fun keyword")
		p.t.Restore(loc)
		return nil, false
	}

	name, ok := p.t.ExpectIdentifier()
	if !ok {
		p.markErr("expected identifier")
		p.t.Restore(loc)
		return nil, false
	}
	n.Name = name

	// expect zero arguments for now
	if _, ok := p.t.Expect(lexer.KLParen); !ok {
		p.markErr("expected open parenthesis")
		p.t.Restore(loc)
		return nil, false
	}
	if _, ok := p.t.Expect(lexer.KRParen); !ok {
		p.markErr("expected closing parenthesis to match open parenthesis")
		p.t.Restore(loc)
		return nil, false
	}

	if _, ok := p.t.Expect(lexer.KArrow); !ok {
		p.markErr("expected arrow")
		p.t.Restore(loc)
		return nil, false
	}

	returnType, ok := p.parseType()
	if !ok {
		p.t.Restore(loc)
		return nil, false
	}
	returnType.Parent = n
	n.Signature.Result = returnType

	body, ok := p.parseBlock()
	if !ok {
		p.t.Restore(loc)
		return nil, false
	}

	// flatten the parsed block into the function body
	n.List = body.List
	for _, child := range n.List {
		child.Parent = n
	}

	return n, true
}
