package ssa

import (
	"fmt"

	"github.com/chenota/acc/internal/buildcfg"
)

// lowerValue rewrites v into its target's machine op, reporting whether it did
type lowerValue func(v *Value) bool

// lower swaps every generic op with a machine form for that form on f's target
func lower(f *Func) error {
	rewrite, err := lowerValueFor(f.Target.Arch)
	if err != nil {
		return err
	}

	for v := range f.UnorderedValues() {
		rewrite(v)
	}

	return nil
}

func lowerValueFor(arch buildcfg.Arch) (lowerValue, error) {
	switch arch {
	case buildcfg.ArchX64:
		return lowerValueX64, nil
	case buildcfg.ArchARM64:
		return lowerValueARM64, nil
	}
	return nil, fmt.Errorf("no lowering rules for %s", arch)
}
