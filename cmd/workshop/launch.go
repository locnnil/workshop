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
	"slices"

	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
)

type CmdLaunch struct {
	waitMixin
	root        *CmdRoot
	WaitOnError bool
	Continue    bool
	Abort       bool
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "launch <WORKSHOP>...",
		Short:   "Construct one or many workshops using their definitions",
		GroupID: GrpCRUD,
		Long: `
This command constructs the workshops listed as arguments by going over their
definitions and installing their components. For each workshop, it:

- Checks the workshop definition and identifies necessary actions

- Retrieves the required components, such as base and SDKs

- Runs SDK setup hooks to initialize the working state

- On success, ties the workshop to the project and starts it


The "--wait-on-error" option pauses the launch if an error occurs.
Thus, you can fix the error and resume the operation or abort and revert it.
This option can only be used with a single workshop.
If multiple workshops are listed and an error occurs,
the operation is aborted and no workshops are constructed.
Also, if you change the workshop definition while fixing the error,
you must abort the operation and restart from scratch.


Notes:

- Names listed as arguments must match respective "name:" values in definitions.

- To update an existing workshop, use "workshop refresh" instead.

- SDKs are installed in the order they are listed in the definition.
`,
		Example: `
Launch the "nimble" and "jazzy" workshops in the current project directory:
$ workshop launch nimble jazzy

The name is optional if the project has only one workshop:
$ workshop launch`,
		RunE:              c.Run,
		ValidArgsFunction: c.complete,
	}

	cmd.PersistentFlags().BoolVar(&c.WaitOnError, "wait-on-error",
		false,
		`Pause the operation on error; to resume, use "--continue" or "--abort".`)
	cmd.PersistentFlags().BoolVar(&c.Continue, "continue",
		false,
		"Continue the previously paused operation.")
	cmd.PersistentFlags().BoolVar(&c.Abort, "abort",
		false,
		"Abort the previously paused operation, reverting any changes.")
	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Return the change ID, don't wait for the operation to finish.")
	cmd.PersistentFlags().BoolVar(&c.verbose, "verbose",
		false,
		"Combine stdout and stderr output from hooks.")

	cmd.MarkFlagsMutuallyExclusive("abort", "continue", "wait-on-error")

	return cmd
}

// waitingChangeError renders the message for an abort or continue requested
// when no launch is in progress to resume.
func (c *CmdLaunch) waitingChangeError() error {
	verb := "abort"
	if c.Continue {
		verb = "continue"
	}
	return fmt.Errorf("cannot %s: no launch in progress", verb)
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	av = strutil.Deduplicate(av)

	cli, err := c.root.client()
	if err != nil {
		return err
	}

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

	mode := "transactional"
	if c.WaitOnError {
		mode = "wait-on-error"
	}
	if c.Continue {
		mode = "continue"
	}
	if c.Abort {
		mode = "abort"
	}

	changeId, err := cli.Launch(project.Id, av, mode, c.verbose)

	var conflictErr client.ChangeConflictError
	switch {
	case err == nil:
	case errors.Is(err, client.ErrorNoWaitingChange):
		return c.waitingChangeError()
	case errors.As(err, &conflictErr):
		return fmt.Errorf(
			"cannot launch %[1]q: %[2]s change is in progress",
			conflictErr.Workshop,
			conflictErr.ChangeKind,
		)
	default:
		return err
	}

	_, err = c.wait(cli, changeId)

	switch {
	case err == nil:
		fmt.Fprintf(Stdout, "%s launched\n", strutil.Quoted(av))
	case errors.Is(err, errNoWait):
	case errors.Is(err, errUndone):
		fmt.Fprintf(Stdout, "%s launch aborted\n", strutil.Quoted(av))
	case errors.Is(err, errWaitOnError):
		w := workshopName(av[0])
		return fmt.Errorf(`cannot complete launch for %q, execution is paused

To proceed, resolve the issue and run "workshop launch --continue %s"
To cancel and undo: "workshop launch --abort %s"
To view more information: "workshop tasks %s"`, w, w, w, changeId)
	default:
		return fmt.Errorf("%v\n%s launch aborted", err, strutil.Quoted(av))
	}

	return nil
}

func (c *CmdLaunch) complete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cli, err := c.root.noRetryClient()
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	project, err := cli.Project(c.root.project())
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	instances, files, err := cli.List(&client.ListOptions{ProjectId: project.Id})
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var workshops []string
	for _, file := range files {
		isInstance := false
		for _, instance := range instances {
			if file.Name == instance.Name {
				isInstance = true
				break
			}
		}
		if !isInstance && !slices.Contains(args, file.Name) {
			workshops = append(workshops, file.Name)
		}
	}

	return workshops, cobra.ShellCompDirectiveNoFileComp
}
