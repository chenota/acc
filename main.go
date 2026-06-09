package main

import (
	"os"

	"github.com/chenota/acc/cmd"
	"github.com/chenota/acc/internal/diagnostic"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		diagnostic.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
