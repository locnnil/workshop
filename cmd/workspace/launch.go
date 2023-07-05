package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/dirs"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type CmdLaunch struct {
	waitMixin
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "launch <workspace>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Launch one or many workspaces",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	var clientConfig client.Config
	var err error

	_, clientConfig.Socket = dirs.GetEnvPaths()
	cli, err := client.New(&clientConfig)
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

	if _, err := c.wait(changeId); err != nil {
		if err == noWait {
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
