package asmtxt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chenota/acc/internal/codegen"
	"github.com/chenota/acc/internal/register"
)

// Stringify transforms a list of instructions into AT&T strings
func Stringify(instrs []codegen.Inst) []string {
	strs := make([]string, 0, len(instrs))

	for _, instr := range instrs {
		if strings.HasSuffix(instr.Op, ":") {
			strs = append(strs, instr.Op)
			continue
		}

		src1 := argText(instr.Src1)
		src2 := argText(instr.Src2)
		dest := argText(instr.Dest)

		if src1 == "" {
			strs = append(strs, fmt.Sprintf("\t%s %s", instr.Op, dest))
		} else if src1 != "" && src2 == "" {
			strs = append(strs, fmt.Sprintf("\t%s %s, %s", instr.Op, src1, dest))
		} else if src1 != "" && src2 != "" {
			strs = append(strs, fmt.Sprintf("\t%s %s, %s, %s", instr.Op, src1, src2, dest))
		}
	}

	return strs
}

func argText(arg codegen.Arg) string {
	switch arg.Kind {
	case codegen.KImmediate:
		return "$" + strconv.FormatInt(int64(arg.Value.(int32)), 10)
	case codegen.KRegister:
		return registerString(arg.Reg, arg.Value.(int))
	case codegen.KStack:
		rbpName := registerString(register.RegBP, 8)
		return fmt.Sprintf("%d(%s)", arg.Value, rbpName)
	case codegen.KText:
		return arg.Value.(string)
	}
	return ""
}

var classicRegs = map[register.Register][]string{
	register.RegA:  {"al", "ax", "eax", "rax"},
	register.RegB:  {"bl", "bx", "ebx", "rbx"},
	register.RegC:  {"cl", "cx", "ecx", "rcx"},
	register.RegD:  {"dl", "dx", "edx", "rdx"},
	register.RegSI: {"sil", "si", "esi", "rsi"},
	register.RegDI: {"dil", "di", "edi", "rdi"},
	register.RegSP: {"spl", "sp", "esp", "rsp"},
	register.RegBP: {"bpl", "bp", "ebp", "rbp"},
}

func registerString(reg register.Register, size int) string {
	var regStr string

	if reg >= register.RegA && reg <= register.RegBP {
		switch size {
		case 1:
			regStr = classicRegs[reg][0]
		case 2:
			regStr = classicRegs[reg][1]
		case 4:
			regStr = classicRegs[reg][2]
		case 8:
			regStr = classicRegs[reg][3]
		}
	}

	if reg >= register.Reg8 && reg <= register.Reg15 {
		switch size {
		case 1:
			regStr = fmt.Sprintf("r%db", reg)
		case 2:
			regStr = fmt.Sprintf("r%dw", reg)
		case 4:
			regStr = fmt.Sprintf("r%dd", reg)
		case 8:
			regStr = fmt.Sprintf("r%d", reg)
		}
	}

	if regStr != "" {
		return "%" + regStr
	}
	return ""
}
