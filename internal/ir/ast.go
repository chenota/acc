package ir

import (
	"iter"
	"maps"

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
	OpRef
	OpDeref
)

type Signature struct {
	Name   *Node
	Params []*Node
	Result *Node

	Label        string
	ClosureCount int
	captures     map[*Sym]struct{}
}

func NewSignature() *Signature {
	return &Signature{
		captures: make(map[*Sym]struct{}),
	}
}

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

// Ident returns the identifier name carried by an OpIdent node.
func (n *Node) Ident() string {
	if n == nil || n.Op != OpIdent {
		return ""
	}

	name, _ := n.Val.(string)
	return name
}

// Children yields every node hanging off n
func (n *Node) Children() iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		if n == nil {
			return
		}

		if n.Signature != nil {
			if n.Signature.Name != nil && !yield(n.Signature.Name) {
				return
			}
			for _, param := range n.Signature.Params {
				if param != nil && !yield(param) {
					return
				}
			}
			if n.Signature.Result != nil && !yield(n.Signature.Result) {
				return
			}
		}

		for _, child := range n.List {
			if child != nil && !yield(child) {
				return
			}
		}
	}
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

func (n *Node) IsLValue() bool {
	if n == nil {
		return false
	}

	return n.Op == OpIdent || n.Op == OpDeref
}

// Encl returns the node of this node's enclosing function
func (n *Node) Encl() *Node {
	return n.Predecessor(OpFunction)
}

func (n *Node) Capture(sy *Sym) {
	// done capturing
	if n == nil || sy.Def == n || sy.Kind == SymFunc {
		return
	}
	// capture in self
	n.Signature.captures[sy] = struct{}{}
	// capture in direct enclosing function
	n.Encl().Capture(sy)
}

func (n *Node) Captures() iter.Seq[*Sym] {
	if n == nil || n.Signature == nil {
		return func(func(*Sym) bool) {}
	}
	return maps.Keys(n.Signature.captures)
}
