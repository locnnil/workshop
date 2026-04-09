package main

import (
	"errors"
	"fmt"

	"github.com/canonical/x-go/strutil"
	"github.com/spf13/cobra"
)

type CmdRestore struct {
	waitMixin
	root *CmdRoot
}

func (c *CmdRestore) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "restore [flags] <WORKSHOP>...",
		Short:   "Restore workshops to the state of the last launch or refresh",
		GroupID: GrpCRUD,
		Long: `
Restore the container filesystem to the point of the last launch or refresh,
then reset the connections and mounts to default settings.
`,
		Example: `
Restore the "nimble" and "jazzy" workshops in the current project directory:
$ workshop restore nimble jazzy

The name is optional if the project has only one workshop:
$ workshop restore`,
		RunE:              c.Run,
		ValidArgsFunction: c.root.completeWorkshopNames([]string{"Ready", "Stopped"}),
	}

	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Return the change ID, don't wait for the operation to finish.")
	cmd.PersistentFlags().BoolVar(&c.verbose, "verbose",
		false,
		"Combine stdout and stderr output from hooks.")

	return cmd
}

func (c *CmdRestore) Run(cmd *cobra.Command, av []string) error {
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

	changeId, err := cli.Restore(project.Id, av, c.verbose)
	if err != nil {
		return err
	}

	_, err = c.wait(cli, changeId)

	switch {
	case err == nil:
		fmt.Fprintf(Stdout, "%s restored\n", strutil.Quoted(av))
	case errors.Is(err, errNoWait):
	case errors.Is(err, errUndone):
		fmt.Fprintf(Stdout, "%s restore aborted\n", strutil.Quoted(av))
	default:
		return fmt.Errorf("%v\n%s restore aborted", err, strutil.Quoted(av))
	}

	return nil
}
