package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/timeutil"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type CmdTasks struct {
	clientMixin
}

func (c *CmdTasks) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "tasks change-id",
		Args:  cobra.RangeArgs(1, 1),
		Short: "Show a summary of tasks for the change",
		RunE:  c.Run,
	}

	return cmd
}

const line = "......................................................................"

func (c *CmdTasks) Run(cmd *cobra.Command, av []string) error {
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

	change, err := c.client.Change(av[0])
	if err != nil {
		return err
	}

	if change != nil {
		tasks := change.Tasks

		slices.SortFunc(tasks, func(a, b *client.Task) bool { return a.SpawnTime.Before(b.SpawnTime) })

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

			for _, tsk := range tasks {
				if len(tsk.Log) == 0 {
					continue
				}
				fmt.Fprintln(os.Stdout)
				fmt.Fprintln(os.Stdout, line)
				fmt.Fprintln(os.Stdout, tsk.Summary)
				fmt.Fprintln(os.Stdout)
				for _, line := range tsk.Log {
					fmt.Fprintln(os.Stdout, line)
				}
			}
		}
	} else {
		return fmt.Errorf("change with id %q not found", av[0])
	}

	return nil
}
