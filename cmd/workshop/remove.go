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
		Short: "Remove one or many workshops",
		Long: `
This command removes the workshops listed as arguments. For each workshop, it:

- Checks that the workshop isn't *Off* or *Pending*
- Stops the workshop if it's not already *Stopped*
- Deletes the workshop but preserves its definition

Notes:
- If any listed workshop is *Off* or *Pending*, none are removed
- To rebuild a removed workshop from scratch, use 'workshop launch'
- For content interface plugs, non-default sources set by 'workshop remount'
  aren't removed
`,

		RunE: c.Run,
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
