package ir

import (
	"github.com/chenota/acc/internal/diagnostic"
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
	OpPlus
	OpMinus
	OpTimes
	OpDiv
	OpIdent
	OpDeclaration
	OpAssignment
	OpNegate
	OpPlusEq
	OpMinusEq
	OpTimesEq
	OpDivEq
	OpCall
)

type Node struct {
	Parent *Node

	Op   Op
	Type *types.Type
	Pos  diagnostic.Pos

	List []*Node

	Signature *Signature

	Sym *Sym

	Val any
}

type Signature struct {
	Name   *Node
	Params []*Node
	Result *Node
}

// Ident returns the identifier name carried by an OpIdent node.
func (n *Node) Ident() string {
	if n.Op != OpIdent {
		return ""
	}

	name, _ := n.Val.(string)
	return name
}

// Predecessor finds the node's closest predecessor with the given op type
func (n *Node) Predecessor(op Op) *Node {
	if n == nil {
		return nil
	}

	curr := n.Parent
	for curr != nil {
		if curr.Op == op {
			return curr
		}
		curr = curr.Parent
	}

	return nil
}
