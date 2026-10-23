//go:build !darwin

package codegen

// ELF C symbols have no prefix
const symbolPrefix = ""

// ELF keeps .L labels out of the symbol table
const localPrefix = ".L"
