package main

import (
	"fmt"
	"strings"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/dirs"
	"github.com/canonical/workspace/internal/timeutil"
	"github.com/spf13/cobra"
)

type CmdTasks struct {
	clientMixin
}

func (c *CmdTasks) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "tasks change-id",
		Args:  cobra.MinimumNArgs(1),
		Short: "Show a summary of tasks for the change",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdTasks) Run(cmd *cobra.Command, av []string) error {
	var clientConfig client.Config
	var clientOpts client.ChangesOptions
	var err error

	_, clientConfig.Socket = dirs.GetEnvPaths()
	cli, err := client.New(&clientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}
	c.setClient(cli)

	if cmd.Parent().Flag("project").Changed {
		clientOpts.ProjectPath = cmd.Parent().Flag("project").Value.String()
	}

	clientOpts.Selector = client.ChangesAll

	change, err := c.client.Change(av[0])
	if err != nil {
		return err
	}

	if change != nil {
		tasks := change.Tasks

		if len(tasks) > 0 {
			w := tabWriter()
			fmt.Fprintf(w, "ID\tStatus\tSpawn\tReady\tSummary\n")

			for _, tsk := range tasks {
				spawnTime := timeutil.Human(tsk.SpawnTime)
				readyTime := timeutil.Human(tsk.ReadyTime)
				if tsk.ReadyTime.IsZero() {
					readyTime = "-"
				}

				fmt.Fprintln(w, strings.Join([]string{
					tsk.ID,
					tsk.Status,
					spawnTime,
					readyTime,
					tsk.Summary}, "\t"))
			}
			w.Flush()
		}
	} else {
		return fmt.Errorf("change with id %q not found", av[0])
	}

	return nil
}
