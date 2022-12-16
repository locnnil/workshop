package main

import (
	"github.com/spf13/cobra"
)

func main() {
	app := &cobra.Command{
		Use:           "workspace",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	app.AddCommand((&CmdLaunch{}).Command())

	if err := app.Execute(); err != nil {
		return
	}
}
