package parser

import (
	"strings"

	"github.com/chenota/acc/internal/diagnostic"
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
)

func (p *parser) parseExpr() (*ir.Node, bool) {
	loc := p.t.Mark()

	expr, err := p.expr(0)
	if err != nil {
		// important we restore the position here so unknown errors are marked at beginning of expression
		p.t.Restore(loc)
		p.markErrValue(err)
		return nil, false
	}

	return expr, true
}

func (p *parser) expr(currentBindingPower int) (*ir.Node, error) {
	leftToken, ok := p.t.Peek()
	if !ok {
		return nil, diagnostic.NewError(p.t.Pos(), "expected prefix or literal expression")
	}
	p.t.Advance()

	leftNode, err := p.nud(leftToken)
	if err != nil {
		return nil, err
	}

	for currentBindingPower < p.nextTokenBindingPower() {
		operator, ok := p.t.Peek()
		if !ok {
			break
		}

		if !isOperator(operator) {
			break
		}

		p.t.Advance()

		prospectiveLeftNode, err := p.led(leftNode, operator)
		if err != nil {
			return nil, err

		}
		leftNode = prospectiveLeftNode
	}

	return leftNode, nil
}

func (p *parser) nud(left lexer.Token) (*ir.Node, error) {
	switch left.Kind {
	case lexer.KInteger:
		intVal, ok := left.ParseInteger()
		if !ok {
			return nil, diagnostic.NewError(left.Pos, "cannot parse integer literal")
		}

		return &ir.Node{
			Op:  ir.OpInt,
			Pos: left.Pos,
			Val: intVal,
		}, nil
	case lexer.KLParen:
		var elist []*ir.Node
		var hasComma bool

		for {
			if _, ok := p.t.Expect(lexer.KRParen); ok {
				break
			}

			e, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			elist = append(elist, e)

			if _, ok := p.t.Expect(lexer.KComma); ok {
				hasComma = true
				continue
			}

			if _, ok := p.t.Expect(lexer.KRParen); !ok {
				return nil, diagnostic.NewError(p.t.Pos(), "expected ',' or closing parenthesis ')' to match opening parenthesis")
			}
			break
		}

		if len(elist) == 0 {
			return &ir.Node{
				Op:  ir.OpUnit,
				Pos: left.Pos,
			}, nil
		} else if len(elist) == 1 && !hasComma {
			return elist[0], nil
		} else {
			return &ir.Node{
				Op:   ir.OpTuple,
				Pos:  left.Pos,
				List: elist,
			}, nil
		}
	case lexer.KIdentifier:
		return &ir.Node{
			Op:  ir.OpIdent,
			Pos: left.Pos,
			Val: left.Text,
		}, nil
	case lexer.KMinus:
		e, err := p.expr(unaryBindingPower)
		if err != nil {
			return nil, err
		}

		return &ir.Node{
			Op:   ir.OpNegate,
			Pos:  left.Pos,
			List: []*ir.Node{e},
		}, nil
	case lexer.KRef:
		e, err := p.expr(unaryBindingPower)
		if err != nil {
			return nil, err
		}

		return &ir.Node{
			Op:   ir.OpRef,
			Pos:  left.Pos,
			List: []*ir.Node{e},
		}, nil
	case lexer.KStar:
		e, err := p.expr(unaryBindingPower)
		if err != nil {
			return nil, err
		}

		return &ir.Node{
			Op:   ir.OpDeref,
			Pos:  left.Pos,
			List: []*ir.Node{e},
		}, nil
	case lexer.KFunKw:
		// return fun keyword back to token stream
		p.t.Retreat()
		f, ok := p.parseFunction()
		if !ok {
			return nil, diagnostic.NewError(left.Pos, "unable to parse function")
		}
		return f, nil
	case lexer.KIfKw:
		// return if keyword back to token stream
		p.t.Retreat()
		c, ok := p.parseConditional(lexer.KIfKw)
		if !ok {
			return nil, diagnostic.NewError(left.Pos, "unable to parse conditional")
		}
		return c, nil
	case lexer.KLoopKw:
		// return loop keyword back to token stream
		p.t.Retreat()
		l, ok := p.parseLoop()
		if !ok {
			return nil, diagnostic.NewError(left.Pos, "unable to parse loop")
		}
		return l, nil
	case lexer.KNilKw:
		return &ir.Node{
			Op:  ir.OpNil,
			Pos: left.Pos,
		}, nil
	case lexer.KTrueKw, lexer.KFalseKw:
		return &ir.Node{
			Op:  ir.OpBool,
			Pos: left.Pos,
			Val: left.Kind == lexer.KTrueKw,
		}, nil
	case lexer.KNot:
		e, err := p.expr(unaryBindingPower)
		if err != nil {
			return nil, err
		}

		return &ir.Node{
			Op:   ir.OpNot,
			Pos:  left.Pos,
			List: []*ir.Node{e},
		}, nil
	default:
		return nil, diagnostic.NewError(left.Pos, "expected prefix or literal expression")
	}
}

func (p *parser) led(left *ir.Node, op lexer.Token) (*ir.Node, error) {
	switch {
	case bopFrom(op) != ir.OpUnknown:
		var rightBindingPower int
		if isRightAssociative(op) {
			rightBindingPower = bindingPower(op) - 1
		} else {
			rightBindingPower = bindingPower(op)
		}

		right, err := p.expr(rightBindingPower)
		if err != nil {
			return nil, err
		}

		// comparisons don't chain
		if isComparison(op) {
			if next, ok := p.t.Peek(); ok && isComparison(next) {
				return nil, diagnostic.NewError(next.Pos, "comparison operators cannot be chained")
			}
		}

		return &ir.Node{
			Op:   bopFrom(op),
			List: []*ir.Node{left, right},
			Pos:  left.Pos,
		}, nil
	case op.Kind == lexer.KLParen:
		n := &ir.Node{
			Op:   ir.OpCall,
			Pos:  left.Pos,
			List: []*ir.Node{left},
		}

		for {
			if _, ok := p.t.Expect(lexer.KRParen); ok {
				break
			}

			arg, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, arg)

			// a comma allows another argument or a closing parenthesis
			if _, ok := p.t.Expect(lexer.KComma); ok {
				continue
			}

			if _, ok := p.t.Expect(lexer.KRParen); !ok {
				return nil, diagnostic.NewError(p.t.Pos(), "expected ',' or closing parenthesis ')' to match opening parenthesis")
			}
			break
		}

		return n, nil
	case op.Kind == lexer.KDot:
		tok, ok := p.t.Peek()
		if !ok {
			return nil, diagnostic.NewError(op.Pos, "empty dot operator")
		}
		p.t.Advance()

		n := &ir.Node{
			Op:   ir.OpDot,
			Pos:  left.Pos,
			List: []*ir.Node{left},
		}

		switch tok.Kind {
		case lexer.KInteger:
			if strings.Contains(tok.Text, "_") {
				return nil, diagnostic.NewError(tok.Pos, "integer literals in dot operators cannot contain underscores")
			}
			fallthrough
		case lexer.KIdentifier:
			right, err := p.nud(tok)
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, right)
		default:
			return nil, diagnostic.NewError(tok.Pos, "invalid token for dot access: %v", tok)
		}

		return n, nil
	default:
		return nil, diagnostic.NewError(op.Pos, "expected infix operator")
	}
}

func (p *parser) nextTokenBindingPower() int {
	if next, ok := p.t.Peek(); ok {
		return bindingPower(next)
	}
	return 0
}

func isRightAssociative(lexer.Token) bool {
	return false
}

func isComparison(t lexer.Token) bool {
	switch t.Kind {
	case lexer.KEqEq, lexer.KNotEq, lexer.KLess, lexer.KLessEq, lexer.KGreater, lexer.KGreaterEq:
		return true
	default:
		return false
	}
}

func isOperator(t lexer.Token) bool {
	return bopFrom(t) != ir.OpUnknown || t.Kind == lexer.KLParen || t.Kind == lexer.KDot
}

// unary binding power should be tighter than any binary operator but looser than a call
const unaryBindingPower = 30

func bindingPower(t lexer.Token) int {
	switch t.Kind {
	case lexer.KOr:
		return 4
	case lexer.KAnd:
		return 6
	case lexer.KEqEq, lexer.KNotEq, lexer.KLess, lexer.KLessEq, lexer.KGreater, lexer.KGreaterEq:
		return 8
	case lexer.KPlus, lexer.KMinus:
		return 10
	case lexer.KStar, lexer.KDiv:
		return 20
	// function call, dot operator
	case lexer.KLParen, lexer.KDot:
		return 40
	default:
		return 0
	}
}

func bopFrom(t lexer.Token) ir.Op {
	switch t.Kind {
	case lexer.KPlus:
		return ir.OpPlus
	case lexer.KMinus:
		return ir.OpMinus
	case lexer.KStar:
		return ir.OpTimes
	case lexer.KDiv:
		return ir.OpDiv
	case lexer.KOr:
		return ir.OpOr
	case lexer.KAnd:
		return ir.OpAnd
	case lexer.KEqEq:
		return ir.OpEqual
	case lexer.KNotEq:
		return ir.OpNotEqual
	case lexer.KLess:
		return ir.OpLess
	case lexer.KLessEq:
		return ir.OpLessEqual
	case lexer.KGreater:
		return ir.OpGreater
	case lexer.KGreaterEq:
		return ir.OpGreaterEqual
	default:
		return ir.OpUnknown
	}
}
