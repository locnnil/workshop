package main

import (
	"fmt"
	"os"

	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/timeutil"
	"github.com/spf13/cobra"
)

type CmdChanges struct {
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
	overlord, err := overlord.New(nil, os.Stdout)
	if err != nil {
		return err
	}

	st := overlord.State()
	st.Lock()
	defer st.Unlock()

	all_changes := st.Changes()
	var changes = make([]*state.Change, 0, len(all_changes))

	/* Only report changes for a certain project */
	if cmd.Parent().Flag("project").Changed {
		for _, chg := range all_changes {
			var project = projectstate.ProjectKey{Path: "-", ProjectId: "-"}
			err := chg.Get("project-key", &project)
			if err == nil {
				if project.Path == Project {
					changes = append(changes, chg)
				}
			}
		}
	} else {
		changes = all_changes
	}

	if len(changes) > 0 {
		w := tabWriter()
		fmt.Fprintf(w, "ID\tStatus\tSpawn\tReady\tProject\tSummary\n")

		for _, chg := range changes {
			var project = projectstate.ProjectKey{Path: "-", ProjectId: "-"}
			chg.Get("project-key", &project)

			spawnTime := timeutil.Human(chg.SpawnTime())
			readyTime := timeutil.Human(chg.ReadyTime())
			if chg.ReadyTime().IsZero() {
				readyTime = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				chg.ID(),
				chg.Status().String(),
				spawnTime,
				readyTime,
				contractHomeDirectory(project.Path),
				chg.Summary())
		}
		w.Flush()
	}

	return nil
}
