package main

import (
	"os"

	"github.com/chenota/acc/cmd"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
