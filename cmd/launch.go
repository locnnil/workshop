package main

import (
	workspace "github.com/canonical/workspace/internal"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type CmdLaunch struct {
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "launch [workspace-name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Launch a workspace",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	fs := afero.NewOsFs()
	_, err := workspace.NewWorkspace(fs, ".workspace.project.yaml")
	return err
}
