package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/timeutil"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type CmdChanges struct {
	clientMixin
}

func (c *CmdChanges) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "changes",
		Args:  cobra.NoArgs,
		Short: "Show a summary of recent changes to the system's workspaces",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdChanges) Run(cmd *cobra.Command, av []string) error {
	var clientOpts client.ChangesOptions
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}
	c.setClient(cli)

	if cmd.Parent().Flag("project").Changed {
		clientOpts.ProjectPath = cmd.Parent().Flag("project").Value.String()
	}

	clientOpts.Selector = client.ChangesAll

	chngs, err := c.client.Changes(&clientOpts)
	if err != nil {
		return err
	}

	slices.SortFunc(chngs, func(a, b *client.Change) bool { return a.SpawnTime.Before(b.SpawnTime) })

	if len(chngs) > 0 {
		w := tabWriter()
		fmt.Fprintf(w, "ID\tStatus\tSpawn\tReady\tSummary\n")

		for _, chg := range chngs {
			spawnTime := timeutil.Human(chg.SpawnTime)
			readyTime := timeutil.Human(chg.ReadyTime)
			if chg.ReadyTime.IsZero() {
				readyTime = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				chg.ID,
				chg.Status,
				spawnTime,
				readyTime,
				chg.Summary)
		}
		w.Flush()
	}

	return nil
}
