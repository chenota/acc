package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenota/acc/internal/compiler"
	"github.com/spf13/cobra"
)

type app struct {
	outputPath string
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

	return cmd
}

func (a *app) run(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	if a.outputPath == "" {
		a.outputPath = fmt.Sprintf("%s.asm", strings.TrimSuffix(inputPath, filepath.Ext(inputPath)))
	}

	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	outputBytes, err := compiler.Compile(inputBytes)
	if err != nil {
		return err
	}

	// Write to stdout if "-" is the output file
	if a.outputPath == "-" {
		_, err := fmt.Fprintln(os.Stdout, string(outputBytes))
		return err
	}

	return os.WriteFile(a.outputPath, outputBytes, 0777)
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
