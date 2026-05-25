package ir

import (
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

type Op int

const (
	OpUnknown Op = iota
	OpFunction
	OpBlock
	OpStmt
	OpExpr
	OpInt
	OpReturn
	OpType
	OpParam
)

type Node struct {
	Parent *Node

	Op   Op
	Type *types.Type
	Pos  lexer.Pos

	List  []*Node
	Left  *Node
	Right *Node

	Name      string
	Signature *Signature

	Sym *Sym

	Val any
}

type Signature struct {
	Params []*Node
	Result *Node
}

func (n *Node) FindPredecessor(op Op) *Node {
	curr := n.Parent

	for curr != nil {
		if curr.Op == op {
			return curr
		}
		curr = curr.Parent
	}

	return nil
}
