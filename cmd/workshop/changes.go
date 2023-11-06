package main

import (
	"fmt"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/timeutil"
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
		Short: "List recent changes to the workshops in a project.",
		Long: `
Any substantial operation on a workshop is a *change* that consists of *tasks*;
the command lists details of recent changes for all workshops within a project.
For each change, it prints the following details:

- ID:      uniquely identifies the change within the project
- Status:  reflects the change's progress and affects the workshop's status
- Spawn:   tells when the change was started
- Ready:   tells when the change was *successfully* finished, if at all
- Summary: lists actions, affected workshops, other information

Notes:
- Only successful changes display values in the *Ready* column
- To investigate the details of a specific change, use 'workshop tasks' instead
`,

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
