package compiler

import (
	"io"

	"github.com/chenota/acc/internal/assemble"
	"github.com/chenota/acc/internal/buildcfg"
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

	if err := buildcfg.Resolve(config.target); err != nil {
		return err
	}

	tokens, err := lexer.Tokenize(r.Reader, lexer.WithFileName(r.Name))
	if err != nil {
		return err
	}

	ast, err := parser.ParseProgram(tokens)
	if err != nil {
		return err
	}

	allFuncs, err := semantic.Analyze(ast)
	if err != nil {
		return err
	}

	ssaValues, err := ssa.BuildAndAllocate(allFuncs)
	if err != nil {
		return err
	}

	src, err := codegen.GenerateAssembly(ssaValues)
	if err != nil {
		return err
	}

	if config.isAssembly {
		_, err := io.WriteString(w, src)
		return err
	}

	if err := assemble.Assemble(src, w); err != nil {
		return err
	}

	return nil
}

type FileDetail struct {
	Reader io.Reader
	Name   string
}
