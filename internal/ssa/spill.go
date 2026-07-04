package ssa

import (
	"math"

	"github.com/chenota/acc/internal/register"
)

// spill lowers register pressure using MIN algorithm
func spill(f *Func) {
	timeline := f.OrderedValues()

	// every position where a value is read as an operand
	uses := make(map[*Value][]int)
	for p, v := range timeline {
		for _, a := range v.Args {
			uses[a] = append(uses[a], p)
		}
	}

	s := &spiller{
		inReg: make(map[*Value]struct{}),
		slot:  make(map[*Value]*Value),
		uses:  uses,
	}

	for p, v := range timeline {
		// operands needed by this instruction must never be chosen as spill victims
		s.pinned = make(map[*Value]struct{})
		for _, a := range v.Args {
			s.pinned[a] = struct{}{}
		}

		// reload every spilled operand
		for i, a := range v.Args {
			if !needsRegister(a) || s.resident(a) {
				continue
			}
			s.makeRoom(f, v, p)
			reload := f.insertValueBefore(v, OpLoad, a.Type, v.Block)
			reload.Args = []*Value{s.slot[a]}
			v.Args[i] = reload
			s.inReg[reload] = struct{}{}
			s.pinned[reload] = struct{}{}
		}

		// operands with no further use are ejected
		for _, a := range v.Args {
			if s.nextUse(a, p) == math.MaxInt {
				delete(s.inReg, a)
			}
		}

		// this value becomes resident
		if needsRegister(v) {
			s.makeRoom(f, v, p)
			s.inReg[v] = struct{}{}
		}
	}
}

type spiller struct {
	inReg  map[*Value]struct{}
	slot   map[*Value]*Value
	pinned map[*Value]struct{}
	uses   map[*Value][]int
}

func (s *spiller) resident(v *Value) bool {
	_, ok := s.inReg[v]
	return ok
}

// makeRoom evicts a resident value if the register file is full
func (s *spiller) makeRoom(f *Func, before *Value, p int) {
	// if there are available regs then don't do anything
	if len(s.inReg) < register.Reserved.Complement().Count() {
		return
	}

	// pick furthest-use value
	var victim *Value
	far := -1
	for cand := range s.inReg {
		if _, keep := s.pinned[cand]; keep {
			continue // needed by the current instruction
		}
		if d := s.nextUse(cand, p); d > far {
			far, victim = d, cand
		}
	}
	if victim == nil {
		return
	}

	if _, done := s.slot[victim]; !done {
		alloca := f.newValue(OpAlloca, victim.Type, victim.Block)
		s.slot[victim] = alloca
		store := f.insertValueBefore(before, OpStore, victim.Type, before.Block)
		store.Args = []*Value{victim, alloca}
	}
	delete(s.inReg, victim)
}

func (s *spiller) nextUse(v *Value, after int) int {
	for _, u := range s.uses[v] {
		if u > after {
			return u
		}
	}
	return math.MaxInt
}

// needsRegister reports whether a value needs a physical register
func needsRegister(v *Value) bool {
	return v.Op != OpLiteral && v.Op != OpAlloca
}
