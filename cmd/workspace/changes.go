package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/timeutil"
	"github.com/spf13/cobra"
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
	var clientConfig client.Config
	var clientOpts client.ChangesOptions
	var err error

	_, clientConfig.Socket = util.GetEnvPaths()
	cli, err := client.New(&clientConfig)
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

	if len(chngs) > 0 {
		w := tabWriter()
		fmt.Fprintf(w, "ID\tStatus\tSpawn\tReady\tProject\tSummary\n")

		for _, chg := range chngs {
			var prj = project.Project{Path: "-", ProjectId: "-"}
			chg.Get("project-key", &prj)

			spawnTime := timeutil.Human(chg.SpawnTime)
			readyTime := timeutil.Human(chg.ReadyTime)
			if chg.ReadyTime.IsZero() {
				readyTime = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				chg.ID,
				chg.Status,
				spawnTime,
				readyTime,
				contractHomeDirectory(prj.Path),
				chg.Summary)
		}
		w.Flush()
	}

	return nil
}
