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
)

type Node struct {
	Parent *Node

	Op   Op
	Type *types.Type
	Pos  diagnostic.Pos

	List []*Node

	Name      string
	Signature *Signature

	Sym *Sym

	Val any
}

type Signature struct {
	Params []*Node
	Result *Node
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

func (n *Node) ScopedSym(name string) *Sym {
	if n == nil {
		return nil
	}

	curr := n.Parent
	for curr != nil {
		if curr.Sym != nil && curr.Sym.Name == name {
			return curr.Sym
		}
		curr = curr.Parent
	}

	return nil
}
