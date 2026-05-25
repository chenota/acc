package parser

import (
	"errors"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
)

func ParseProgram(t *lexer.TokenList) ([]*ir.Node, error) {
	var globalStmts []*ir.Node

	// consume statements until we can't
	for {
		if s, ok := parseStmt(t); ok {
			globalStmts = append(globalStmts, s)
		} else {
			break
		}
	}

	// assert no leftover tokens
	if !t.Empty() {
		return nil, errors.New("token list not empty")
	}

	// vertial slice check: for now we should have a single function
	// eventually we'll separate statments into types, configure global vars, create a shadow function, etc.
	if len(globalStmts) != 1 || globalStmts[0].Op != ir.OpFunction {
		return nil, errors.New("should have a single function named 'main'")
	}

	return globalStmts, nil
}

func parseBlock(t *lexer.TokenList) (*ir.Node, bool) {
	loc := t.Mark()

	n := &ir.Node{
		Op:  ir.OpBlock,
		Pos: t.Pos(),
	}

	_, ok := t.Expect(lexer.KindLBracket)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	var stmts []*ir.Node
	for {
		s, ok := parseStmt(t)
		if !ok {
			break
		}
		s.Parent = n
		stmts = append(stmts, s)
	}
	n.List = stmts

	_, ok = t.Expect(lexer.KindRBracket)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return n, true
}

func parseStmt(t *lexer.TokenList) (*ir.Node, bool) {
	if f, ok := parseFunction(t); ok {
		return f, true
	}

	return parseReturn(t)
}

func parseReturn(t *lexer.TokenList) (*ir.Node, bool) {
	loc := t.Mark()

	n := &ir.Node{
		Op:  ir.OpReturn,
		Pos: t.Pos(),
	}

	if _, ok := t.Expect(lexer.KindReturnKw); !ok {
		t.Restore(loc)
		return nil, false
	}

	e, ok := parseExpr(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}
	e.Parent = n
	n.List = []*ir.Node{e}

	if _, ok = t.Expect(lexer.KindSemicolon); !ok {
		t.Restore(loc)
		return nil, false
	}

	return n, true
}

func parseFunction(t *lexer.TokenList) (*ir.Node, bool) {
	loc := t.Mark()
	pos := t.Pos()

	n := &ir.Node{
		Op:        ir.OpFunction,
		Pos:       pos,
		Signature: &ir.Signature{},
	}

	_, ok := t.Expect(lexer.KindFunKw)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	name, ok := t.ExpectIdentifier()
	if !ok {
		t.Restore(loc)
		return nil, false
	}
	n.Name = name

	// expect zero arguments for now
	if _, ok := t.Expect(lexer.KindLParen); !ok {
		t.Restore(loc)
		return nil, false
	}
	if _, ok := t.Expect(lexer.KindRParen); !ok {
		t.Restore(loc)
		return nil, false
	}

	if _, ok := t.Expect(lexer.KindArrow); !ok {
		t.Restore(loc)
		return nil, false
	}

	returnType, ok := parseType(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}
	returnType.Parent = n
	n.Signature.Result = returnType

	body, ok := parseBlock(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	// flatten the parsed block into the function body
	n.List = body.List
	for _, child := range n.List {
		child.Parent = n
	}

	return n, true
}
