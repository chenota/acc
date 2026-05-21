package ast

import (
	"github.com/chenota/acc/internal/src"
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
	Op   Op
	Type *types.Type
	Pos  src.Pos

	List  []*Node
	Left  *Node
	Right *Node

	Name      string
	Signature *Signature

	Val any
}

type Signature struct {
	Params []*Node
	Result *Node
}
