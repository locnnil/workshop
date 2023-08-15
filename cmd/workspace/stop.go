package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/spf13/cobra"
)

type CmdStop struct {
	waitMixin
}

func (c *CmdStop) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "stop <workspace>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Stop one or many workspaces",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdStop) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)
	c.skipAbort = true

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	changeId, err := c.client.Stop(project.Id, av)
	if err != nil {
		return err
	}

	if _, err := c.wait(changeId, false); err != nil {
		if err == errNoWait {
			return nil
		}
		return err
	}

	return nil
}
