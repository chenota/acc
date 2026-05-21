package parser

import (
	"errors"

	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func ParseProgram(t *lexer.TokenList) ([]*ast.Node, error) {
	fun, ok := parseFunction(t)
	if !ok {
		return nil, errors.New("could not parse function")
	}
	if fun.Val.(string) != "main" {
		return nil, errors.New("expected function name to be 'main'")
	}
	return []*ast.Node{fun}, nil
}

func parseFunction(t *lexer.TokenList) (*ast.Node, bool) {
	loc := t.Mark()

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

	funType, ok := parseType(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	// make sure this is a function type
	if funType.Type.Kind != types.KFunction {
		t.Restore(loc)
		return nil, false
	}

	body, ok := parseBlock(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Node{
		Op:   ast.OpFunction,
		Type: funType.Type,
		List: body.List, // flatten the parsed block into the function body
		Val:  name,      // store just the name for now we might need more info later
	}, true
}

func parseBlock(t *lexer.TokenList) (*ast.Node, bool) {
	loc := t.Mark()

	_, ok := t.Expect(lexer.KindLBracket)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	var stmts []*ast.Node
	for {
		s, ok := parseStmt(t)
		if !ok {
			break
		}
		stmts = append(stmts, s)
	}

	_, ok = t.Expect(lexer.KindRBracket)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Node{
		Op:   ast.OpBlock,
		List: stmts,
	}, true
}

func parseStmt(t *lexer.TokenList) (*ast.Node, bool) {
	loc := t.Mark()

	_, ok := t.Expect(lexer.KindReturnKw)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	e, ok := parseExpr(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	_, ok = t.Expect(lexer.KindSemicolon)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Node{
		Op:   ast.OpReturn,
		List: []*ast.Node{e},
	}, true
}
