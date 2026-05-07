package compiler

import (
	"fmt"
)

// Compile is the top-level function of the acc compiler.
// It orchestrates all the compiler's important components.
func Compile(inputBytes []byte) ([]byte, error) {
	// TODO: Do something with the input file
	fmt.Println(string(inputBytes))

	// TODO: Output some assembly
	return []byte("Some compiled code..."), nil
}
