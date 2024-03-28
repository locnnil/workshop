package main

import (
	"fmt"
	"path/filepath"

	"github.com/canonical/workshop/client"
	"github.com/spf13/cobra"
)

type CmdRemount struct {
	waitMixin
}

func (c *CmdRemount) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "remount <WORKSHOP>/<SDK>:<PLUG> <SOURCE>",
		Args:  cobra.ExactArgs(2),
		Short: "Mount a new source location to the content interface plug's target.",
		Long: `
This command mounts a new source location on the host to the target directory
of the specified content interface plug, qualified by the SDK name.
Specifically, it does the following:

- Attempts the mount operation atomically;
  this normally succeeds if the new source is either a non-existing directory
  or an empty directory on the same file system as the current source
- Otherwise, performs the mount operation only if the workshop is *Stopped*
  to prevent data corruption

Notes:
- To stop the workshop, use 'workshop stop'
- 'workshop info' explicitly lists any remounted plugs for a workshop
- 'workshop refresh' mounts the last source set by 'workshop remount', if any
- During 'workshop remove', non-default sources set by 'workshop remount'
  aren't removed
`,

		RunE: c.Run,
	}

	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Do not wait for the operation to finish but just print the change id")

	return cmd
}

func (c *CmdRemount) Run(cmd *cobra.Command, av []string) error {
	var err error

	plugRef, err := client.ParsePlugRef(av[0])
	if err != nil {
		return err
	}

	source, err := filepath.Abs(av[1])
	if err != nil {
		return err
	}

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)
	defer func() {
		if cli != nil {
			maybePresentWarnings(cli.WarningsSummary())
		}
	}()

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	plugRef.ProjectId = project.Id

	changeId, err := c.client.Remount(plugRef, source)
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
