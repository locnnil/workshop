package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/spf13/cobra"
)

type CmdRemove struct {
	waitMixin
}

func (c *CmdRemove) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "remove <WORKSPACE>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Remove one or many workspaces",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdRemove) Run(cmd *cobra.Command, av []string) error {
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

	changeId, err := c.client.Remove(project.Id, av)
	if err != nil {
		return err
	}

	if _, err := c.wait(changeId, false); err != nil {
		if err == errNoWait {
			return nil
		}
		return err
	}

	for _, name := range av {
		fmt.Fprintf(Stdout, "%s removed\n", name)
	}

	return nil
}
