package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"
)

type CmdStart struct {
	waitMixin
}

func (c *CmdStart) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "start <WORKSHOP>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Start one or many workspaces.",
		Long: `
This command activates the workspaces listed as arguments. For each one, it:

- Makes sure the workshop was actually launched
- Activates the workshop for use and sets it to *Ready*

If multiple workspaces are listed and an error occurs,
the operation is aborted and no workspaces are started.

Notes:
- If a workshop is already started or wasn't yet launched, an error occurs
- When interrupted, the command attempts to gracefully revert its actions
- To stop a started workshop, use 'workshop stop'
`,
		RunE: c.Run,
	}

	return cmd
}

func (c *CmdStart) Run(cmd *cobra.Command, av []string) error {
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

	for _, name := range av {
		fmt.Fprintf(Stdout, "%q started\n", name)
	}

	return nil
}
