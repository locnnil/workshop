package main

import (
	"fmt"

	"github.com/canonical/workshop/client"
	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"
)

type CmdStop struct {
	waitMixin
}

func (c *CmdStop) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "stop <WORKSHOP>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Stop one or many workspaces.",
		Long: `
This command deactivates the workspaces listed as arguments. For each one, it:

- Makes sure the workshop was actually started or is already stopped
- Deactivates the workshop and sets it to *Stopped*

If multiple workspaces are listed and an error occurs,
the operation is aborted and no workspaces are stopped.

Notes:
- If a workshop wasn't yet started or even launched, an error occurs
- When interrupted, the command attempts to gracefully revert its actions
- To start a stopped workshop, use 'workshop start'
`,
		RunE: c.Run,
	}

	return cmd
}

func (c *CmdStop) Run(cmd *cobra.Command, av []string) error {
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

	for _, name := range av {
		fmt.Fprintf(Stdout, "%q stopped\n", name)
	}
	return nil
}
