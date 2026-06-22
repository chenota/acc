package compiler

import (
	"fmt"
	"io"

	"github.com/chenota/acc/internal/asmtxt"
	"github.com/chenota/acc/internal/assemble"
	"github.com/chenota/acc/internal/codegen"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/ssa"
)

// Compile is the top-level function of the acc compiler.
// It orchestrates all the compiler's important components.
func Compile(r FileDetail, w io.Writer, opts ...Option) error {
	config := compilerOptions{}

	for _, o := range opts {
		o(&config)
	}

	tokens, err := lexer.Tokenize(r.Reader, lexer.WithFileName(r.Name))
	if err != nil {
		return err
	}

	ast, err := parser.ParseProgram(tokens)
	if err != nil {
		return err
	}

	if err := semantic.Analyze(ast); err != nil {
		return err
	}

	ssaValues, err := ssa.BuildAndAllocate(ast)
	if err != nil {
		return err
	}

	instructions, err := codegen.GenerateProgram(ssaValues)
	if err != nil {
		return err
	}

	stringInstructions, err := asmtxt.Stringify(instructions)
	if err != nil {
		return err
	}

	if config.isAssembly {
		for _, inst := range stringInstructions {
			_, err := fmt.Fprintln(w, inst)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if err := assemble.Assemble(stringInstructions, w); err != nil {
		return err
	}

	return nil
}

type FileDetail struct {
	Reader io.Reader
	Name   string
}
