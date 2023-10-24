package main

import (
	"fmt"

	"github.com/canonical/workshop/client"
	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"
)

type CmdRemove struct {
	waitMixin
}

func (c *CmdRemove) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "remove <WORKSHOP>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Remove one or many workspaces",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdRemove) Run(cmd *cobra.Command, av []string) error {
	var err error

	av = strutil.Deduplicate(av)

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
		fmt.Fprintf(Stdout, "%q removed\n", name)
	}

	return nil
}
