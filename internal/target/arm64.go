package target

import "github.com/chenota/acc/internal/buildcfg"

const (
	RegX0 Register = iota
	RegX1
	RegX2
	RegX3
	RegX4
	RegX5
	RegX6
	RegX7
	RegX8
	RegX9
	RegX10
	RegX11
	RegX12
	RegX13
	RegX14
	RegX15
	RegX16
	RegX17
	RegX18
	RegX19
	RegX20
	RegX21
	RegX22
	RegX23
	RegX24
	RegX25
	RegX26
	RegX27
	RegX28
	RegX29
	RegX30
	RegXSP
)

var ARM64AAPCS = &Target{
	Arch: buildcfg.ArchARM64,

	// x16 and x17 are left for codegen's scratch since linker veneers can clobber them across a call anyway
	Allocatable: NewMask(
		RegX0, RegX1, RegX2, RegX3, RegX4, RegX5, RegX6, RegX7,
		RegX8, RegX9, RegX10, RegX11, RegX12, RegX13, RegX14, RegX15,
		RegX18, RegX19, RegX20, RegX21, RegX22, RegX23, RegX24, RegX25, RegX26, RegX27, RegX28,
	),
	CallerSaved: NewMask(
		RegX0, RegX1, RegX2, RegX3, RegX4, RegX5, RegX6, RegX7,
		RegX8, RegX9, RegX10, RegX11, RegX12, RegX13, RegX14, RegX15,
		RegX16, RegX17, RegX18,
	),
	CalleeSaved: NewMask(RegX19, RegX20, RegX21, RegX22, RegX23, RegX24, RegX25, RegX26, RegX27, RegX28),

	Args:    []Register{RegX0, RegX1, RegX2, RegX3, RegX4, RegX5, RegX6, RegX7},
	Results: []Register{RegX0, RegX1},
	// AAPCS64 has no static chain register, so take the first temporary past the arguments and the indirect result register
	ClosureContext: RegX9,

	StackPointer: RegXSP,
	FramePointer: RegX29,
	LinkRegister: RegX30,

	FrameRecordSize: 16,
}

var ARM64Apple = &Target{
	// x18 is reserved on Apple machines

	Arch: buildcfg.ArchARM64,

	// x16 and x17 are left for codegen's scratch since linker veneers can clobber them across a call anyway
	Allocatable: NewMask(
		RegX0, RegX1, RegX2, RegX3, RegX4, RegX5, RegX6, RegX7,
		RegX8, RegX9, RegX10, RegX11, RegX12, RegX13, RegX14, RegX15,
		RegX19, RegX20, RegX21, RegX22, RegX23, RegX24, RegX25, RegX26, RegX27, RegX28,
	),
	CallerSaved: NewMask(
		RegX0, RegX1, RegX2, RegX3, RegX4, RegX5, RegX6, RegX7,
		RegX8, RegX9, RegX10, RegX11, RegX12, RegX13, RegX14, RegX15,
		RegX16, RegX17,
	),
	CalleeSaved: NewMask(RegX19, RegX20, RegX21, RegX22, RegX23, RegX24, RegX25, RegX26, RegX27, RegX28),

	Args:    []Register{RegX0, RegX1, RegX2, RegX3, RegX4, RegX5, RegX6, RegX7},
	Results: []Register{RegX0, RegX1},
	// AAPCS64 has no static chain register, so take the first temporary past the arguments and the indirect result register
	ClosureContext: RegX9,

	StackPointer: RegXSP,
	FramePointer: RegX29,
	LinkRegister: RegX30,

	FrameRecordSize: 16,
}
