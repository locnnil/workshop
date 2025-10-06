package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/logger"
)

func main() {
	l, err := logger.New(Stderr, 0)
	if err != nil {
		panic(err)
	}
	logger.SetLogger(l)

	cmd := (&CmdRoot{}).Command()
	// Work around https://github.com/spf13/cobra/issues/2257.
	cmd.SetArgs(slices.Clone(os.Args[1:]))

	if err := cmd.Execute(); err != nil {
		exitErr, ok := err.(*client.ExitError)
		if ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
