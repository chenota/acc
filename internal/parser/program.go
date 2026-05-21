package parser

import (
	"errors"

	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func ParseProgram(t *lexer.TokenList) ([]*ast.Function, error) {
	fun, ok := parseFunction(t)
	if !ok {
		return nil, errors.New("could not parse function")
	}
	if fun.Name != "main" {
		return nil, errors.New("expected function name to be 'main'")
	}
	return []*ast.Function{fun}, nil
}

func parseFunction(t *lexer.TokenList) (*ast.Function, bool) {
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

	// parseType returns a generic type and we need this to be a function type so cast it
	funTypeCast, ok := funType.(types.Function)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	body, ok := parseBlock(t)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Function{
		Name: name,
		Type: funTypeCast,
		Body: body,
	}, true
}

func parseBlock(t *lexer.TokenList) (*ast.Block, bool) {
	loc := t.Mark()

	_, ok := t.Expect(lexer.KindLBracket)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	var stmts []ast.Stmt
	var seenReturn bool // don't add more statements to the block if we've seen a return
	for {
		s, ok := parseStmt(t)
		if !ok {
			break
		}
		if !seenReturn {
			stmts = append(stmts, s)
		}

		if _, ok = s.(*ast.StmtReturn); ok {
			seenReturn = true
		}
	}

	_, ok = t.Expect(lexer.KindRBracket)
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return &ast.Block{Statements: stmts}, true
}

func parseStmt(t *lexer.TokenList) (ast.Stmt, bool) {
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

	return &ast.StmtReturn{Expr: e}, true
}
