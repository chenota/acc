//go:build !darwin

package codegen

// ELF C symbols have no prefix
const symbolPrefix = ""
