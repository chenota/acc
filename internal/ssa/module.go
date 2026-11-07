package ssa

import (
	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/types"
)

// Module is the flat namespace of top-level symbols the object file will contain.
type Module struct {
	Funcs  []*Func
	Target *register.Target
	byName map[string]*Func
}

// declare adds a named function shell to the pool.
func (m *Module) declare(name string, sig *types.Type) *Func {
	f := newFunc(name, sig)
	f.Target = m.Target
	m.Funcs = append(m.Funcs, f)
	m.byName[name] = f
	return f
}

// lookup resolves a named function reference, or nil if there is none.
func (m *Module) lookup(name string) *Func {
	return m.byName[name]
}
