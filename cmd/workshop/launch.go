package main

import (
	"fmt"

	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"
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
		Use:   "launch <WORKSHOP>...",
		Short: "Construct one or many workshops using their definitions",
		Long: `
This command constructs the workshops listed as arguments by going over their
definitions and installing their components. For each workshop, it:

- Checks the workshop definition and identifies necessary actions

- Retrieves the required components, such as base and SDKs

- Runs SDK setup hooks to initialise the working state

- On success, ties the workshop to the project and starts it


The '--wait-on-error' option pauses the launch if an error occurs.
Thus, you can fix the error and resume the operation or abort and revert it.
This option can only be used with a single workshop.
If multiple workshops are listed and an error occurs,
the operation is aborted and no workshops are constructed.


Notes:

- Names listed as arguments must match respective 'name:' values in definitions.

- To update an existing workshop, use 'workshop refresh' instead.

- SDKs are installed in the order they are listed in the definition.
`,
		Example: `
Launch the 'nimble' and 'jazzy' workshops in the current project directory:
$ workshop launch nimble jazzy

The name is optional if the project has only one workshop:
$ workshop launch`,
		RunE: c.Run,
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
	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Return the change ID, don't wait for the operation to finish")

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	av = strutil.Deduplicate(av)

	if c.Abort && c.Continue {
		return fmt.Errorf("cannot launch: '--abort' incompatible with '--continue'")
	}

	if c.WaitOnError && c.Abort {
		return fmt.Errorf("cannot launch: '--wait-on-error' incompatible with '--abort'")
	}

	if c.WaitOnError && c.Continue {
		return fmt.Errorf("cannot launch: '--wait-on-error' incompatible with '--continue'")
	}

	// We should have no more than one argument (a single workshop) for a
	// wait-on-error operation
	if (c.Abort || c.Continue || c.WaitOnError) && len(av) > 1 {
		return fmt.Errorf("cannot launch: '--wait-on-error' incompatible with multiple workshops")
	}

	cli, err := c.root.client()
	if err != nil {
		return err
	}

	project, err := cli.Project(c.root.project)
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

	changeId, err := cli.Launch(project.Id, av, mode)
	if err != nil {
		return err
	}

	if _, err := c.wait(cli, changeId); err != nil {
		if err == errNoWait {
			return nil
		}
		if err == errWaitOnError {
			return fmt.Errorf("cannot launch; fix the errors reported,\n"+
				"then run \"workshop launch --continue %s\".\n"+
				"To abort and revert, run \"workshop launch --abort %s\"", workshopName(av[0]), workshopName(av[0]))
		}
		return fmt.Errorf("%v\n%s launch aborted", err, strutil.Quoted(av))
	}

	if c.Abort {
		fmt.Fprintf(Stdout, "%q launch aborted\n", av[0])
		return nil
	}

	for _, i := range av {
		fmt.Fprintf(Stdout, "%q launched\n", i)
	}

	return nil
}
