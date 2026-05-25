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
	Op   Op
	Type *types.Type
	Pos  lexer.Pos

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
