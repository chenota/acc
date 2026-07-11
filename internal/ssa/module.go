package ssa

// Module is the flat namespace of top-level symbols the object file will contain.
type Module struct {
	Funcs  []*Func
	byName map[string]*Func
}

func newModule() *Module {
	return &Module{byName: make(map[string]*Func)}
}

// declare adds a named function shell to the pool.
func (m *Module) declare(name string) *Func {
	f := &Func{Name: name}
	m.Funcs = append(m.Funcs, f)
	m.byName[name] = f
	return f
}

// lookup resolves a named function reference, or nil if there is none.
func (m *Module) lookup(name string) *Func {
	return m.byName[name]
}
