package ast

import (
	"math/big"

	"github.com/chenota/acc/internal/types"
)

type Program struct {
	Functions []Function
}

type Function struct {
	Name string
	Type types.Function
	Body Block
}

type Block struct {
	Statements []Stmt
}

type Stmt interface {
	isStmt()
}

type StmtReturn struct {
	Expr Expr
}

func (s StmtReturn) isStmt() {}

type Expr interface {
	isExpr()
	Type() types.Type
}

type ExprInt struct {
	Value *big.Int
	Size  types.IntSize
}

func (e ExprInt) isExpr() {}

func (e ExprInt) Type() types.Type {
	return types.Int{Size: e.Size}
}
