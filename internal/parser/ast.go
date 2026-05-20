package parser

import "math/big"

type Program struct {
	Functions []Function
}

type Function struct {
	Name string
	Type TypeFunction
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
}

// ExprInt holds an unsized integer value.
type ExprInt struct {
	Value *big.Int
}

func (e ExprInt) isExpr() {}

type Type interface {
	isType()
}

type AtomKind int

const (
	AtomKindInt AtomKind = iota
)

type TypeAtom struct {
	Kind AtomKind
}

func (t TypeAtom) isType() {}

type TypeFunction struct {
	Inputs []Type
	Output Type
}

func (t TypeFunction) isType() {}

type TypeUnit struct{}

func (t TypeUnit) isType() {}
