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

	// consume functions until we can't
	for {
		if s, ok := p.parseFunction(); ok {
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
	if n, ok := p.parseDeclaration(); ok {
		return n, true
	}

	if n, ok := p.parseAssignment(); ok {
		return n, true
	}

	if n, ok := p.parseAssignmentOp(); ok {
		return n, true
	}

	return p.parseReturn()
}

func (p *parser) parseAssignmentOp() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Pos: p.t.Pos(),
	}

	varName, ok := p.t.ExpectIdentifier()
	if !ok {
		p.markErr("expected identifier in assignment operation")
		p.t.Restore(loc)
		return nil, false
	}
	n.Name = varName

	opToken, ok := p.t.Peek()
	if !ok {
		p.markErr("unexpected EOF")
		p.t.Restore(loc)
		return nil, false
	}

	var op ir.Op
	switch opToken.Kind {
	case lexer.KPlusEq:
		op = ir.OpPlusEq
	case lexer.KMinusEq:
		op = ir.OpMinusEq
	case lexer.KMulEq:
		op = ir.OpTimesEq
	case lexer.KDivEq:
		op = ir.OpDivEq
	default:
		p.markErr("expected an assignment operator")
		p.t.Restore(loc)
		return nil, false
	}
	p.t.Advance()
	n.Op = op

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

func (p *parser) parseDeclaration() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Op:   ir.OpDeclaration,
		Pos:  p.t.Pos(),
		List: make([]*ir.Node, 2),
	}

	if _, ok := p.t.Expect(lexer.KLetKw); !ok {
		p.markErr("expected let keyword")
		p.t.Restore(loc)
		return nil, false
	}

	varName, ok := p.t.ExpectIdentifier()
	if !ok {
		p.markErr("expected identifier in variable declaration")
		p.t.Restore(loc)
		return nil, false
	}
	n.Name = varName

	// first element of the list is either the declared type or nil if unspecified
	if varType, ok := p.parseType(); ok {
		n.List[0] = varType
	} else {
		n.List[0] = nil
	}

	if _, ok := p.t.Expect(lexer.KEqual); !ok {
		p.markErr("expected equal sign in variable declaration")
		p.t.Restore(loc)
		return nil, false
	}

	e, ok := p.parseExpr()
	if !ok {
		p.t.Restore(loc)
		return nil, false
	}
	e.Parent = n
	n.List[1] = e

	if _, ok = p.t.Expect(lexer.KSemicolon); !ok {
		p.markErr("expected semicolon")
		p.t.Restore(loc)
		return nil, false
	}

	return n, true
}

func (p *parser) parseAssignment() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Op:  ir.OpAssignment,
		Pos: p.t.Pos(),
	}

	varName, ok := p.t.ExpectIdentifier()
	if !ok {
		p.markErr("expected identifier")
		p.t.Restore(loc)
		return nil, false
	}
	n.Name = varName

	if _, ok := p.t.Expect(lexer.KEqual); !ok {
		p.markErr("expected equal sign in variable assignment")
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

	if _, ok := p.t.Expect(lexer.KLParen); !ok {
		p.markErr("expected open parenthesis")
		p.t.Restore(loc)
		return nil, false
	}

	var params []*ir.Node
	if param, ok := p.parseParam(); ok {
		param.Parent = n
		params = append(params, param)

		for {
			if _, ok := p.t.Expect(lexer.KComma); !ok {
				break
			}
			if param, ok := p.parseParam(); ok {
				param.Parent = n
				params = append(params, param)
			} else {
				p.markErr("expected function parameter following comma")
				p.t.Restore(loc)
				return nil, false
			}
		}
	}
	n.Signature.Params = params

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

func (p *parser) parseParam() (*ir.Node, bool) {
	loc := p.t.Mark()

	n := &ir.Node{
		Op:  ir.OpParam,
		Pos: p.t.Pos(),
	}

	name, ok := p.t.ExpectIdentifier()
	if !ok {
		p.markErr("expected identifier")
		p.t.Restore(loc)
		return nil, false
	}
	n.Name = name

	paramType, ok := p.parseType()
	if !ok {
		p.t.Restore(loc)
		return nil, false
	}
	paramType.Parent = n
	n.List = []*ir.Node{paramType}

	return n, true
}
