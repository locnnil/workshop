package main

import (
	"fmt"
	"os"

	"github.com/canonical/workspace/internal/logger"
	"github.com/spf13/cobra"
)

// exitStatus can be used in panic(&exitStatus{code}) to cause Workspaces's main
// function to exit with a given exit code, for the rare cases when you want
// to return an exit code other than 0 or 1, or when an error return is not
// possible.
type exitStatus struct {
	code int
}

func (e *exitStatus) Error() string {
	return fmt.Sprintf("internal error: exitStatus{%d} being handled as normal error", e.code)
}

var workspaced = &cobra.Command{
	Use:              "workspace",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var (
	osExit = os.Exit
)

func main() {
	logger.SetLogger(logger.New(os.Stderr, "[workspaced] "))
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(*exitStatus); ok {
				osExit(e.code)
			}
			panic(v)
		}
	}()

	workspaced.AddCommand((&cmdRun{}).Command())
	workspaced.Execute()
}
