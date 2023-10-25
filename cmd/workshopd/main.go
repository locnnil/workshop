package main

import (
	"fmt"
	"os"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
	"github.com/spf13/cobra"
)

// exitStatus can be used in panic(&exitStatus{code}) to cause Workshops's main
// function to exit with a given exit code, for the rare cases when you want
// to return an exit code other than 0 or 1, or when an error return is not
// possible.
type exitStatus struct {
	code int
}

func (e *exitStatus) Error() string {
	return fmt.Sprintf("internal error: exitStatus{%d} being handled as normal error", e.code)
}

var workshopd = &cobra.Command{
	Use:              "workshop",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var (
	osExit = os.Exit
)

func main() {
	l, err := logger.New(os.Stderr, 0)
	if err != nil {
		panic(err)
	}
	logger.SetLogger(l)
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(*exitStatus); ok {
				osExit(e.code)
			}
			panic(v)
		}
	}()

	if err := dirs.CreateDirs(); err != nil {
		panic(err)
	}

	workshopd.AddCommand((&cmdRun{}).Command())
	if err = workshopd.Execute(); err != nil {
		os.Exit(1)
	}
}
