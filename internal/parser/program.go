package parser

import (
	"errors"

	"github.com/chenota/acc/internal/lexer"
)

func ParseProgram(t *lexer.TokenList) ([]Function, error) {
	fun, ok := parseFunction(t)
	if !ok {
		return nil, errors.New("could not parse function")
	}
	if fun.Name != "main" {
		return nil, errors.New("expected function name to be 'main'")
	}
	return []Function{fun}, nil
}

func parseFunction(t *lexer.TokenList) (Function, bool) {
	loc := t.Mark()

	_, ok := t.Expect(lexer.KindFunKw)
	if !ok {
		t.Restore(loc)
		return Function{}, false
	}

	name, ok := t.ExpectIdentifier()
	if !ok {
		t.Restore(loc)
		return Function{}, false
	}

	funType, ok := parseType(t)
	if !ok {
		t.Restore(loc)
		return Function{}, false
	}

	// parseType returns a generic type and we need this to be a function type so cast it
	funTypeCast, ok := funType.(TypeFunction)
	if !ok {
		t.Restore(loc)
		return Function{}, false
	}

	body, ok := parseBlock(t)
	if !ok {
		t.Restore(loc)
		return Function{}, false
	}

	return Function{
		Name: name,
		Type: funTypeCast,
		Body: body,
	}, true
}

func parseBlock(t *lexer.TokenList) (Block, bool) {
	loc := t.Mark()

	_, ok := t.Expect(lexer.KindLBracket)
	if !ok {
		t.Restore(loc)
		return Block{}, false
	}

	var stmts []Stmt
	var seenReturn bool // don't add more statements to the block if we've seen a return
	for {
		s, ok := parseStmt(t)
		if !ok {
			break
		}
		if !seenReturn {
			stmts = append(stmts, s)
		}

		if _, ok = s.(StmtReturn); ok {
			seenReturn = true
		}
	}

	_, ok = t.Expect(lexer.KindRBracket)
	if !ok {
		t.Restore(loc)
		return Block{}, false
	}

	return Block{Statements: stmts}, true
}

func parseStmt(t *lexer.TokenList) (Stmt, bool) {
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

	return StmtReturn{Expr: e}, true
}
