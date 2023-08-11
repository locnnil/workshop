package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/spf13/cobra"
)

type CmdStart struct {
	waitMixin
}

func (c *CmdStart) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "start <workspace>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Start one or many workspaces",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdStart) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	changeId, err := c.client.Start(project.Id, av)
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
