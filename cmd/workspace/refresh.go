package main

import (
	"fmt"
	"os"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/dirs"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type CmdRefresh struct {
	waitMixin
	WaitOnError bool
	Continue    bool
	Abort       bool
}

func (c *CmdRefresh) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "refresh <workspace>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Refresh one or many workspaces",
		RunE:  c.Run,
	}

	cmd.PersistentFlags().BoolVar(&c.WaitOnError, "wait-on-error", false, "Stop the refresh operation on error without reverting to the previous state (default behaviour: revert the workspace to the previous state on error)")
	cmd.PersistentFlags().BoolVar(&c.Continue, "continue", false, "Continue the refresh operation from the last failure")
	cmd.PersistentFlags().BoolVar(&c.Abort, "abort", false, "Abort the refresh operation and revert the workspace to the pre-refresh state")

	return cmd
}

func (c *CmdRefresh) Run(cmd *cobra.Command, av []string) error {
	var clientConfig client.Config
	var err error

	if c.Abort && c.Continue {
		return fmt.Errorf("flags --continue and --abort are incompatible")
	}

	if c.WaitOnError && c.Abort {
		return fmt.Errorf("flags --wait-on-error and --abort are incompatible")
	}

	if c.WaitOnError && c.Continue {
		return fmt.Errorf("flags --wait-on-error and --continue are incompatible")
	}

	if (c.Abort || c.Continue || c.WaitOnError) && len(av) > 1 {
		return fmt.Errorf("the wait-on-error mode can be used with a single workspace only")
	}

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

	changeId, err := c.client.Refresh(project.Id, av, mode)
	if err != nil {
		return err
	}

	if _, err := c.wait(changeId, c.Abort); err != nil {
		if err == errNoWait {
			return nil
		}
		if err == errWaitOnError && mode != "transactional" {
			return fmt.Errorf("%q refresh failed, resolve all errors and run \"workspace refresh --continue\".\n"+
				"To abort and get back to the state before run \"workspace refresh --abort\".", av[0])
		}
		return err
	}

	if c.Abort && err == nil {
		fmt.Fprintf(os.Stdout, "%q refresh aborted\n", av[0])
		return nil
	}

	workspaces, err := c.client.ListWorkspaces(&client.ListOptions{ProjectId: project.Id})
	if err != nil {
		return nil
	}

	for _, i := range av {
		if slices.ContainsFunc(workspaces, func(w *client.Workspace) bool { return w.Name == i && w.State == "Ready" }) {
			fmt.Fprintf(os.Stdout, "%q refreshed\n", i)
		}
	}
	return nil
}
