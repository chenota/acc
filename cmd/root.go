package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenota/acc/internal/compiler"
	"github.com/chenota/acc/internal/diagnostic"
	"github.com/spf13/cobra"
)

type app struct {
	outputPath string
	isAssembly bool
	isStatic   bool
}

// NewRootCommand creates a self-contained Cobra command to run the app
func NewRootCommand() *cobra.Command {
	app := &app{}

	cmd := &cobra.Command{
		Use:   "acc [file]",
		Short: "Compiler for the acc language.",
		Args:  validatePositionalArgs,
		RunE:  app.run,
	}

	cmd.Flags().StringVarP(&app.outputPath, "output", "o", "", "path to the output file")
	cmd.Flags().BoolVarP(&app.isAssembly, "asm", "S", false, "output AMD64 assembly")
	cmd.Flags().BoolVar(&app.isStatic, "static", false, "compile into a static self-contained binary")

	return cmd
}

func (a *app) run(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	var input io.ReadCloser
	if inputPath == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(inputPath)
		if err != nil {
			return err
		}
		input = f
	}
	defer input.Close()

	var output io.WriteCloser
	if a.outputPath == "-" {
		output = os.Stdout
	} else {
		// ensure defaults if empty
		if a.outputPath == "" {
			extension := "out"
			if a.isAssembly {
				extension = "s"
			}
			a.outputPath = fmt.Sprintf("%s.%s", strings.TrimSuffix(inputPath, filepath.Ext(inputPath)), extension)
		}
		f, err := os.Create(a.outputPath)
		if err != nil {
			return err
		}
		output = f
	}
	defer output.Close()

	var opts []compiler.Option

	if a.isAssembly {
		opts = append(opts, compiler.WithAssemblyOnly())
	}

	if a.isStatic {
		opts = append(opts, compiler.WithStaticCompilation())
	}

	inputName := inputPath
	if inputName == "-" {
		inputName = "stdin"
	}

	err := compiler.Compile(compiler.FileDetail{Reader: input, Name: inputName}, output, opts...)
	if err != nil {
		diagnostic.PrintError(os.Stderr, err)
	}

	return nil
}

func validatePositionalArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return err
	}

	extension := filepath.Ext(args[0])
	if extension != ".acc" {
		return fmt.Errorf("invalid file extension: expected '.acc', got '%s'", extension)
	}

	return nil
}
