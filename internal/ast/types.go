package ast

import (
	"github.com/chenota/acc/internal/src"
	"github.com/chenota/acc/internal/types"
)

type Op int

const (
	OpFunction Op = iota
	OpBlock
	OpStmt
	OpExpr
	OpInt
	OpReturn
	OpType
)

type Node struct {
	Op   Op
	Type *types.Type
	Pos  src.Pos

	List  []*Node
	Left  *Node
	Right *Node

	Val any
}
