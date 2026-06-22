package asmtxt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chenota/acc/internal/codegen"
	"github.com/chenota/acc/internal/register"
)

// Stringify transforms a list of instructions into AT&T strings
func Stringify(instrs []codegen.Inst) ([]string, error) {
	strs := make([]string, 0, len(instrs))

	for _, instr := range instrs {
		if strings.HasSuffix(instr.Op, ":") {
			strs = append(strs, instr.Op)
			continue
		}

		src1, err := argText(instr.Src1)
		if err != nil {
			return nil, fmt.Errorf("instruction %q src1: %w", instr.Op, err)
		}
		src2, err := argText(instr.Src2)
		if err != nil {
			return nil, fmt.Errorf("instruction %q src2: %w", instr.Op, err)
		}
		dest, err := argText(instr.Dest)
		if err != nil {
			return nil, fmt.Errorf("instruction %q dest: %w", instr.Op, err)
		}

		if src1 == "" {
			strs = append(strs, fmt.Sprintf("\t%s %s", instr.Op, dest))
		} else if src2 == "" {
			strs = append(strs, fmt.Sprintf("\t%s %s, %s", instr.Op, src1, dest))
		} else {
			strs = append(strs, fmt.Sprintf("\t%s %s, %s, %s", instr.Op, src1, src2, dest))
		}
	}

	return strs, nil
}

func argText(arg codegen.Arg) (string, error) {
	switch arg.Kind {
	case codegen.KImmediate:
		v, ok := arg.Value.(int32)
		if !ok {
			return "", fmt.Errorf("immediate value has wrong type %T, want int32", arg.Value)
		}
		return "$" + strconv.FormatInt(int64(v), 10), nil
	case codegen.KRegister:
		size, ok := arg.Value.(int)
		if !ok {
			return "", fmt.Errorf("register size has wrong type %T, want int", arg.Value)
		}
		return registerString(arg.Reg, size)
	case codegen.KStack:
		offset, ok := arg.Value.(int)
		if !ok {
			return "", fmt.Errorf("stack offset has wrong type %T, want int", arg.Value)
		}
		rbpName, err := registerString(register.RegBP, 8)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d(%s)", offset, rbpName), nil
	case codegen.KText:
		v, ok := arg.Value.(string)
		if !ok {
			return "", fmt.Errorf("text value has wrong type %T, want string", arg.Value)
		}
		return v, nil
	}
	return "", nil
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

func registerString(reg register.Register, size int) (string, error) {
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
		default:
			return "", fmt.Errorf("unsupported size %d for register %d", size, reg)
		}
	} else if reg >= register.Reg8 && reg <= register.Reg15 {
		switch size {
		case 1:
			regStr = fmt.Sprintf("r%db", reg)
		case 2:
			regStr = fmt.Sprintf("r%dw", reg)
		case 4:
			regStr = fmt.Sprintf("r%dd", reg)
		case 8:
			regStr = fmt.Sprintf("r%d", reg)
		default:
			return "", fmt.Errorf("unsupported size %d for register %d", size, reg)
		}
	} else {
		return "", fmt.Errorf("unknown register %d", reg)
	}

	return "%" + regStr, nil
}
