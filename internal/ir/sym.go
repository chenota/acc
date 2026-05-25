package ir

import "github.com/chenota/acc/internal/types"

type Sym struct {
	Name string
	Def  *Node
	Type *types.Type
}
