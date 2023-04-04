package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/canonical/workspace/internal/overlord"
	"github.com/spf13/cobra"
)

type CmdTasks struct {
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
	overlord, err := overlord.New(nil, os.Stdout)
	if err != nil {
		return err
	}

	st := overlord.State()
	st.Lock()
	defer st.Unlock()

	change := st.Change(av[0])
	if change != nil {
		tasks := change.Tasks()

		if len(tasks) > 0 {
			w := tabWriter()
			fmt.Fprintf(w, "ID\tStatus\tSummary\n")

			for _, tsk := range tasks {
				fmt.Fprintln(w, strings.Join([]string{
					tsk.ID(),
					tsk.Status().String(),
					tsk.Summary()}, "\t"))
			}
			w.Flush()
		}
	} else {
		return fmt.Errorf("change with id %q not found", av[0])
	}

	return nil
}
