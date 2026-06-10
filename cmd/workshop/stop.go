// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"fmt"

	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
)

type CmdStop struct {
	waitMixin
	root *CmdRoot
}

func (c *CmdStop) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "stop <WORKSHOP>...",
		Short:   "Stop one or many workshops",
		GroupID: GrpCRUD,
		Long: `
This command deactivates the workshops listed as arguments. For each one, it:

- Makes sure the workshop was actually started or is already stopped

- Deactivates the workshop and sets it to 'Stopped'


If multiple workshops are listed and an error occurs,
the operation is aborted and no workshops are stopped.


Notes:

- If a workshop wasn't yet started or even launched, an error occurs.

- When interrupted, the command attempts to gracefully revert its actions.

- To start a stopped workshop, use "workshop start".
`,
		Example: `
Stop the "nimble" and "jazzy" workshops in the current project directory:
$ workshop stop nimble jazzy

The name is optional if the project has only one workshop:
$ workshop stop`,
		RunE:              c.Run,
		ValidArgsFunction: c.root.completeWorkshopNames([]string{"Ready"}),
	}

	return cmd
}

func (c *CmdStop) Run(cmd *cobra.Command, av []string) error {
	av = strutil.Deduplicate(av)

	cli, err := c.root.client()
	if err != nil {
		return err
	}

	c.skipAbort = true

	project, err := cli.Project(c.root.project())
	if err != nil {
		return err
	}

	if len(av) == 0 {
		name, err := cli.SingleWorkshopName(project)
		if err != nil {
			return err
		}
		av = []string{name}
	}

	changeId, err := cli.Stop(project.Id, av)

	var conflictErr client.ChangeConflictError
	switch {
	case err == nil:
	case errors.As(err, &conflictErr) && conflictErr.ChangeKind == "refresh":
		return fmt.Errorf(`
cannot stop %[1]q: a refresh task is waiting on error
To view details: "workshop tasks %[2]s"

To abort and undo the refresh: "workshop refresh --abort %[1]s"
Otherwise, resolve the error, then run "workshop refresh --continue %[1]s"
After that, run "workshop stop %[1]s" again.`[1:],
			conflictErr.Workshop,
			conflictErr.ChangeID,
		)
	case errors.As(err, &conflictErr):
		return fmt.Errorf(`
cannot stop %[1]q: tasks are in progress
To view details: "workshop tasks %[2]s"`[1:],
			conflictErr.Workshop,
			conflictErr.ChangeID,
		)
	default:
		return err
	}

	if _, err := c.wait(cli, changeId); err != nil {
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
