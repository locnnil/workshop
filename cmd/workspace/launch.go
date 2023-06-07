package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/dirs"
	"github.com/spf13/cobra"
)

type CmdLaunch struct {
	waitMixin
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "launch workspace-name",
		Args:  cobra.MinimumNArgs(1),
		Short: "Launch a workspace",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	var clientConfig client.Config
	var err error

	_, clientConfig.Socket = dirs.GetEnvPaths()
	cli, err := client.New(&clientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	changeId, err := c.client.Launch(project.Id, av)
	if err != nil {
		return err
	}

	if _, err := c.wait(changeId); err != nil {
		if err == noWait {
			return nil
		}
		return err
	}
	return nil
}
