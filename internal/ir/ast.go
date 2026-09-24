package ir

import (
	"iter"
	"slices"

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
	OpDot
	OpUnit
	OpTuple
	OpNil
)

type Signature struct {
	Name   *Node
	Params []*Node
	Result *Node

	Label        string
	ClosureCount int
	captures     []*Sym
}

func NewSignature() *Signature {
	return &Signature{}
}

type Attribute int

const (
	ARecursive Attribute = iota
)

type Node struct {
	Parent *Node

	Op   Op
	Type *types.Type
	Pos  diagnostic.Pos

	List []*Node

	Signature *Signature

	Sym *Sym

	Attrs map[Attribute]struct{}
	Val   any
}

func (n *Node) SetAttribute(a Attribute) {
	if n.Attrs == nil {
		n.Attrs = make(map[Attribute]struct{})
	}
	n.Attrs[a] = struct{}{}
}

func (n *Node) Attribute(a Attribute) bool {
	_, ok := n.Attrs[a]
	return ok
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

	return n.Op == OpIdent || n.Op == OpDeref || n.Op == OpDot
}

// Encl returns the node of this node's enclosing function
func (n *Node) Encl() *Node {
	return n.Predecessor(OpFunction)
}

func (n *Node) Capture(sy *Sym) {
	// done capturing or already captured
	if n == nil || sy.Def == n || sy.Kind == SymFunc || slices.Contains(n.Signature.captures, sy) {
		return
	}
	// capture in self
	n.Signature.captures = append(n.Signature.captures, sy)
	// capture in direct enclosing function
	n.Encl().Capture(sy)
}

func (n *Node) Captures() []*Sym {
	if n == nil || n.Signature == nil {
		return nil
	}
	return n.Signature.captures
}

func (n *Node) NextClosureCount() int {
	if n == nil || n.Signature == nil {
		return -1
	}
	n.Signature.ClosureCount += 1
	return n.Signature.ClosureCount - 1
}
