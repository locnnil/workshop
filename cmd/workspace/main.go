package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              "workspace",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var Project string

func init() {
	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", ".", "specify a project's directory path")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
}

func main() {
	rootCmd.Execute()
}
