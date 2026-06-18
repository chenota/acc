package parser

import (
	"errors"

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
		if diagnosticErr, ok := errors.AsType[*diagnostic.Error](err); ok {
			p.markErrDiagnostic(diagnosticErr)
		} else {
			p.markErr(err.Error())
		}
		return nil, false
	}

	return expr, true
}

func (p *parser) expr(currentBindingPower int) (*ir.Node, error) {
	leftToken, ok := p.t.Peek()
	if !ok {
		return nil, diagnostic.NewError("expected prefix or literal expression", p.t.Pos())
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
	case lexer.KindInteger:
		intVal, ok := left.ParseInteger()
		if !ok {
			return nil, diagnostic.NewError("cannot parse integer literal", left.Pos)
		}

		return &ir.Node{
			Op:  ir.OpInt,
			Pos: left.Pos,
			Val: intVal,
		}, nil
	case lexer.KindLParen:
		e, err := p.expr(0)
		if err != nil {
			return nil, err
		}

		if _, ok := p.t.Expect(lexer.KindRParen); !ok {
			return nil, diagnostic.NewError("expected closing parenthisis ')' to match opening parenthisis", p.t.Pos())
		}

		return e, nil
	default:
		return nil, diagnostic.NewError("expected prefix or literal expression", left.Pos)
	}
}

func (p *parser) led(left *ir.Node, op lexer.Token) (*ir.Node, error) {
	if bopFrom(op) != ir.OpUnknown {
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

		return &ir.Node{
			Op:   bopFrom(op),
			List: []*ir.Node{left, right},
			Pos:  left.Pos,
		}, nil
	}

	return nil, diagnostic.NewError("expected infix operator", op.Pos)
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

func isOperator(t lexer.Token) bool {
	return bopFrom(t) != ir.OpUnknown
}

func bindingPower(t lexer.Token) int {
	switch t.Kind {
	case lexer.KindPlus, lexer.KindMinus:
		return 10
	case lexer.KindStar, lexer.KindDiv:
		return 20
	default:
		return 0
	}
}

func bopFrom(t lexer.Token) ir.Op {
	switch t.Kind {
	case lexer.KindPlus:
		return ir.OpPlus
	case lexer.KindMinus:
		return ir.OpMinus
	case lexer.KindStar:
		return ir.OpTimes
	case lexer.KindDiv:
		return ir.OpDiv
	default:
		return ir.OpUnknown
	}
}
