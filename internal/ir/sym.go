package ir

import "github.com/chenota/acc/internal/types"

type Table struct {
	parent  *Table
	entries map[string]*Sym
}

type SymKind int

const (
	SymLocal SymKind = iota
	SymGlobal
)

type Sym struct {
	Name string
	Type *types.Type
	Kind SymKind
}

func NewTable() *Table {
	return &Table{
		entries: make(map[string]*Sym),
	}
}

func (t *Table) NewChild() *Table {
	child := NewTable()
	child.parent = t
	return child
}

func (t *Table) Register(name string, symType *types.Type) *Sym {
	if t == nil {
		return nil
	}

	if _, ok := t.entries[name]; ok {
		return nil
	}

	kind := SymLocal
	if t.parent == nil {
		kind = SymGlobal
	}

	t.entries[name] = &Sym{
		Name: name,
		Type: symType,
		Kind: kind,
	}

	return t.entries[name]
}

func (t *Table) Sym(name string) *Sym {
	if t == nil {
		return nil
	}

	if entry, ok := t.entries[name]; ok {
		return entry
	}

	return t.parent.Sym(name)
}
