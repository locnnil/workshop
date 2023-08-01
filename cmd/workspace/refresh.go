package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/spf13/cobra"
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

	cmd.PersistentFlags().BoolVar(&c.WaitOnError, "wait-on-error", false, "Stop the refresh operation on error without reverting to the pre-refresh state (default behaviour: revert the workspace to the pre-refresh state on error)")
	cmd.PersistentFlags().BoolVar(&c.Continue, "continue", false, "Continue the refresh operation from the last point of failure")
	cmd.PersistentFlags().BoolVar(&c.Abort, "abort", false, "Abort the refresh operation and revert the workspace to the pre-refresh state")

	return cmd
}

func (c *CmdRefresh) Run(cmd *cobra.Command, av []string) error {
	var err error

	if c.Abort && c.Continue {
		return fmt.Errorf("cannot refresh: flags --continue and --abort are incompatible")
	}

	if c.WaitOnError && c.Abort {
		return fmt.Errorf("cannot refresh: flags --wait-on-error and --abort are incompatible")
	}

	if c.WaitOnError && c.Continue {
		return fmt.Errorf("cannot refresh: flags --wait-on-error and --continue are incompatible")
	}

	if (c.Abort || c.Continue || c.WaitOnError) && len(av) > 1 {
		return fmt.Errorf("cannot refresh: the wait-on-error mode can be used with a single workspace only")
	}

	cli, err := client.New(&ClientConfig)
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
		if err == errWaitOnError {
			return fmt.Errorf("cannot refresh, resolve all errors and run \"workspace refresh --continue %s\".\n"+
				"To abort and get back to the state before run \"workspace refresh --abort %s\"", av[0], av[0])
		}
		return fmt.Errorf("%v \nRefresh aborted", err)
	}

	if c.Abort && err == nil {
		fmt.Fprintf(Stdout, "%q refresh aborted\n", av[0])
		return nil
	}

	for _, i := range av {
		fmt.Fprintf(Stdout, "%q refreshed\n", i)
	}
	return nil
}
