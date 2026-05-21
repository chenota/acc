package parser

import (
	"errors"

	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func ParseProgram(t *lexer.TokenList) ([]*ast.Node, error) {
	var globalStmts []*ast.Node

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

	// vertial slice check: for now we should have a single "main" function and nothing else
	// eventually we'll separate statments into types, configure global vars, create a shadow function, etc.
	if len(globalStmts) != 1 || globalStmts[0].Op != ast.OpFunction || globalStmts[0].Val.(ast.FunctionData).Name != "main" {
		return nil, errors.New("should have a single function named 'main'")
	}

	return globalStmts, nil
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
	if f, ok := parseFunction(t); ok {
		return f, true
	}

	return parseReturn(t)
}

func parseReturn(t *lexer.TokenList) (*ast.Node, bool) {
	loc := t.Mark()

	if _, ok := t.Expect(lexer.KindReturnKw); !ok {
		t.Restore(loc)
		return nil, false
	}

	e, ok := parseExpr(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	if _, ok = t.Expect(lexer.KindSemicolon); !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Node{
		Op:   ast.OpReturn,
		List: []*ast.Node{e},
	}, true
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

	body, ok := parseBlock(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Node{
		Op:   ast.OpFunction,
		List: body.List, // flatten the parsed block into the function body
		Val: ast.FunctionData{
			Name: name,
			// function declared with no parameters inherently has an anonymous unit parameter; we'll formalize this later
			Params: []ast.Param{{Type: &types.Type{Kind: types.KUnit}}},
			Return: returnType.Type,
		},
	}, true
}
