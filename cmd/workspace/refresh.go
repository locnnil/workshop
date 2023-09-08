package main

import (
	"fmt"

	"github.com/canonical/workspace/client"
	"github.com/canonical/x-go/strutil"
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
		Use:   "refresh [--abort|--continue|--wait-on-error] <WORKSPACE>...",
		Args:  cobra.MinimumNArgs(1),
		Short: "Update workspaces according to their definitions.",
		Long: `
This command updates the workspaces listed as arguments by going over their
definitions once again. For each workspace, it:

- Saves the working state of the workspace
- Checks the workspace definition and identifies any updates required
- Retrieves the updated components
- Applies and verifies the changes to the workspace
- Restores the working state of the workspace

The '--wait-on-error' option pauses the refresh if an error occurs.
Thus, you can fix the error and resume the operation or abort and revert it.
This option can only be used with a single workspace.

If multiple workspaces are listed and an error occurs,
the operation is aborted and reverted for *all* of them.

Notes:
- The workspace must be *Ready* to be refreshed
- Throughout the refresh, all affected workspaces remain *Pending*
- If the refresh removes an SDK from the workspace, the SDK state isn't saved
- Updated and newly added SDKs are installed in alphabetical order
`,

		RunE:  c.Run,
	}

	cmd.PersistentFlags().BoolVar(&c.WaitOnError, "wait-on-error",
		false,
		"Pause the operation on error; to resume, use '--continue' or '--abort'.")
		cmd.PersistentFlags().BoolVar(&c.Continue, "continue",
		false,
		"Continue the previously paused operation.")
		cmd.PersistentFlags().BoolVar(&c.Abort, "abort",
		false,
		"Abort the previously paused operation, reverting any changes.")

	return cmd
}

func (c *CmdRefresh) Run(cmd *cobra.Command, av []string) error {
	var err error

	if c.Abort && c.Continue {
		return fmt.Errorf("cannot refresh: '--abort' incompatible with '--continue'")
	}

	if c.WaitOnError && c.Abort {
		return fmt.Errorf("cannot refresh: '--wait-on-error' incompatible with '--abort'")
	}

	if c.WaitOnError && c.Continue {
		return fmt.Errorf("cannot refresh: '--wait-on-error' incompatible with '--continue'")
	}

	if (c.Abort || c.Continue || c.WaitOnError) && len(av) > 1 {
		return fmt.Errorf("cannot refresh: '--wait-on-error' incompatible with multiple workspaces")
	}

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create the client: %v", err)
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
			return fmt.Errorf("cannot refresh; fix the errors reported by \"workspace info\",\n" +
				"then run \"workspace refresh --continue %s\".\n" +
				"To abort and revert, run \"workspace refresh --abort %s\"", av[0], av[0])
		}

		return fmt.Errorf("%v\n%s refresh aborted", err, strutil.Quoted(av))
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
