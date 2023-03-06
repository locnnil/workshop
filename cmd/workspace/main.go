package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              "workspace",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var Project string

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		panic("cannot get a current working directory")
	}

	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", cwd, "specify a project's directory path")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
	rootCmd.AddCommand((&CmdList{}).Command())
	rootCmd.Execute()
}
