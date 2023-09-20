package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type CmdLaunch struct {
	waitMixin
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "launch <WORKSPACE>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Initialise one or many workspaces using their definitions.",
		Long: `
This command constructs the workspaces listed as arguments by going over their
definitions and installing their components. For each workspace, it:

- Checks the workspace definition and identifies necessary actions
- Retrieves the required components, such as base and SDKs
- Runs SDK setup hooks to initialise the working state
- On success, sets the workspace to *Ready*

If multiple workspaces are listed and an error occurs,
the operation is aborted and no workspaces are constructed.

Notes:
- Names listed as arguments must match respective 'name:' values in definitions
- To update an existing workspace, use 'workspace refresh' instead
- SDKs are installed in alphabetical order
`,

		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	changeId, err := c.client.Launch(project.Id, av)
	if err != nil {
		return err
	}

	if _, err := c.wait(changeId, false); err != nil {
		if err == errNoWait {
			return nil
		}
		return err
	}

	workspaces, err := c.client.ListWorkspaces(&client.ListOptions{ProjectId: project.Id})
	if err != nil {
		return nil
	}
	for _, i := range av {
		if slices.ContainsFunc(workspaces, func(w *client.Workspace) bool { return w.Name == i && w.State == "Ready" }) {
			fmt.Fprintf(Stdout, "%q launched\n", i)
		}
	}

	return nil
}
