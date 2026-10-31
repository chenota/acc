package ssa

import "slices"

// setArgs replaces v's operands, keeping every operand's user list in sync.
func (v *Value) setArgs(args ...*Value) {
	for _, a := range v.Args {
		a.users = removeOne(a.users, v)
	}

	v.Args = args
	for _, a := range args {
		a.users = append(a.users, v)
	}
}

func (v *Value) setArg(i int, a *Value) {
	v.Args[i].users = removeOne(v.Args[i].users, v)
	v.Args[i] = a
	a.users = append(a.users, v)
}

func (v *Value) addArg(a *Value) {
	v.Args = append(v.Args, a)
	a.users = append(a.users, v)
}

func (b *Block) setControl(vals ...*Value) {
	for _, c := range b.Control {
		c.ctrl = removeOne(c.ctrl, b)
	}

	b.Control = vals
	for _, c := range vals {
		c.ctrl = append(c.ctrl, b)
	}
}

// removeOne deletes an x from xs, which must be there, moving xs's last element into its place
func removeOne[T comparable](xs []T, x T) []T {
	i := slices.Index(xs, x)
	last := len(xs) - 1
	xs[i] = xs[last]
	clear(xs[last:])
	return xs[:last]
}

// redirectUses points every operand and control reference to old at new, leaving the instruction stream untouched.
func (f *Func) redirectUses(old, new *Value) {
	if old == new {
		return
	}

	for _, u := range old.users {
		for i, a := range u.Args {
			if a == old {
				u.Args[i] = new
			}
		}
	}
	new.users = append(new.users, old.users...)
	old.users = nil

	for _, b := range old.ctrl {
		for i, c := range b.Control {
			if c == old {
				b.Control[i] = new
			}
		}
	}
	new.ctrl = append(new.ctrl, old.ctrl...)
	old.ctrl = nil
}

// replaceValue substitutes new for old in the instruction stream and redirects all uses of old to new.
func (f *Func) replaceValue(old, new *Value) {
	for _, block := range f.Blocks {
		if i := block.indexOf(old); i >= 0 {
			block.Values[i] = new
		}
	}

	f.redirectUses(old, new)
	old.setArgs()
}

// hasUses reports whether anything other than v itself references v.
func (f *Func) hasUses(v *Value) bool {
	return len(v.ctrl) > 0 || slices.ContainsFunc(v.users, func(u *Value) bool { return u != v })
}

func (f *Func) useCount(v *Value) int {
	return len(v.users) + len(v.ctrl)
}

func (f *Func) removeIfDead(v *Value) {
	if f.hasUses(v) {
		return
	}

	f.removeValue(v)
}

// removeValue drops v from its block and releases its operands; v must have no remaining uses.
func (f *Func) removeValue(v *Value) {
	v.Block.Values = slices.DeleteFunc(v.Block.Values, func(u *Value) bool { return u == v })
	v.setArgs()
}
