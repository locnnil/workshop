package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/logger"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	l, err := logger.New(Stderr, 0)
	if err != nil {
		panic(err)
	}

	logger.SetLogger(l)

	rootCmd := (&CmdRoot{cwd: cwd}).Command()
	// Work around https://github.com/spf13/cobra/issues/2257.
	rootCmd.SetArgs(slices.Clone(os.Args[1:]))

	if err = rootCmd.Execute(); err != nil {
		exitError, ok := err.(*client.ExitError)
		if ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintf(Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
