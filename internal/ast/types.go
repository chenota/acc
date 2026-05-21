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

type FunctionData struct {
	Name   string
	Params []Param
	Return *types.Type
}

type Param struct {
	Name string
	Type *types.Type
}
