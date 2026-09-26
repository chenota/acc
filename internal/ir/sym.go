package ir

import "github.com/chenota/acc/internal/types"

type Table struct {
	parent  *Table
	entries map[string]*Sym
}

type SymKind int

const (
	SymLocal SymKind = iota
	SymFunc
	SymParam
)

// SymAttribute is a property a symbol is declared with
type SymAttribute int

const (
	SAConst SymAttribute = iota
)

// SymObservation is a property analysis discovers from a symbol's uses
type SymObservation int

const (
	SOMutated SymObservation = iota
)

type Sym struct {
	Name string
	Type *types.Type
	Kind SymKind
	Def  *Node

	Attrs map[SymAttribute]struct{}
	Obs   map[SymObservation]struct{}
}

func (s *Sym) SetAttribute(a SymAttribute) {
	if s.Attrs == nil {
		s.Attrs = make(map[SymAttribute]struct{})
	}
	s.Attrs[a] = struct{}{}
}

func (s *Sym) Attribute(a SymAttribute) bool {
	_, ok := s.Attrs[a]
	return ok
}

func (s *Sym) Observe(o SymObservation) {
	if s.Obs == nil {
		s.Obs = make(map[SymObservation]struct{})
	}
	s.Obs[o] = struct{}{}
}

func (s *Sym) Observed(o SymObservation) bool {
	_, ok := s.Obs[o]
	return ok
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

func (t *Table) Register(name string, symType *types.Type, symKind SymKind) *Sym {
	if t == nil {
		return nil
	}

	if _, ok := t.entries[name]; ok {
		return nil
	}

	t.entries[name] = &Sym{
		Name: name,
		Type: symType,
		Kind: symKind,
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
