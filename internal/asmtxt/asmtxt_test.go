package asmtxt

import (
	"testing"

	"github.com/chenota/acc/internal/codegen"
	"github.com/chenota/acc/internal/register"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsmTxt_OnlyDest(t *testing.T) {
	cgInstrs := []codegen.Inst{
		{Op: "movl", Dest: codegen.Arg{Kind: codegen.KImmediate, Value: int32(10)}},
	}

	instrs, err := Stringify(cgInstrs)

	require.NoError(t, err)
	require.Len(t, instrs, len(cgInstrs))
	assert.Equal(t, "\tmovl $10", instrs[0])
}

func TestAsmTxt_OneSrc(t *testing.T) {
	cgInstrs := []codegen.Inst{
		{
			Op:   "movq",
			Src1: codegen.Arg{Kind: codegen.KImmediate, Value: int32(10)},
			Dest: codegen.Arg{Kind: codegen.KRegister, Reg: register.Reg10, Value: 8},
		},
	}

	instrs, err := Stringify(cgInstrs)

	require.NoError(t, err)
	require.Len(t, instrs, len(cgInstrs))
	assert.Equal(t, "\tmovq $10, %r10", instrs[0])
}

func TestAsmTxt_TwoSrc(t *testing.T) {
	cgInstrs := []codegen.Inst{
		{
			Op:   "movl",
			Src1: codegen.Arg{Kind: codegen.KImmediate, Value: int32(10)},
			Src2: codegen.Arg{Kind: codegen.KStack, Value: -8},
			Dest: codegen.Arg{Kind: codegen.KRegister, Reg: register.RegA, Value: 4},
		},
	}

	instrs, err := Stringify(cgInstrs)

	require.NoError(t, err)
	require.Len(t, instrs, len(cgInstrs))
	assert.Equal(t, "\tmovl $10, -8(%rbp), %eax", instrs[0])
}

func TestAsmTxt_Label(t *testing.T) {
	cgInstrs := []codegen.Inst{{Op: "great:"}}

	instrs, err := Stringify(cgInstrs)

	require.NoError(t, err)
	require.Len(t, instrs, len(cgInstrs))
	assert.Equal(t, "great:", instrs[0])
}

func TestAsmTxt_Directive(t *testing.T) {
	cgInstrs := []codegen.Inst{
		{
			Op:   ".hello",
			Dest: codegen.Arg{Kind: codegen.KText, Value: "world"},
		},
	}

	instrs, err := Stringify(cgInstrs)

	require.NoError(t, err)
	require.Len(t, instrs, len(cgInstrs))
	assert.Equal(t, "\t.hello world", instrs[0])
}
