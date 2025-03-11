package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/dirs"
)

type CmdRoot struct {
	cli     *client.Client
	project string
}

func (c *CmdRoot) Command(cwd string) *cobra.Command {
	cmd := &cobra.Command{
		Use: "workshop",
		// Avoid printing errors twice
		SilenceErrors:    true,
		SilenceUsage:     true,
		TraverseChildren: true,

		PersistentPreRunE: c.preRun,
		PersistentPostRun: c.postRun,
	}

	cmd.AddCommand((&CmdLaunch{root: c}).Command())
	cmd.AddCommand((&CmdList{root: c}).Command())
	cmd.AddCommand((&CmdChanges{root: c}).Command())
	cmd.AddCommand((&CmdTasks{root: c}).Command())
	cmd.AddCommand((&CmdRefresh{root: c}).Command())
	cmd.AddCommand((&CmdStart{root: c}).Command())
	cmd.AddCommand((&CmdStop{root: c}).Command())
	cmd.AddCommand((&CmdInfo{root: c}).Command())
	cmd.AddCommand((&CmdExec{root: c}).Command())
	cmd.AddCommand((&CmdShell{root: c}).Command())
	cmd.AddCommand((&CmdRun{root: c}).Command())
	cmd.AddCommand((&CmdScripts{root: c}).Command())
	cmd.AddCommand((&CmdRemove{root: c}).Command())
	cmd.AddCommand((&CmdRemount{root: c}).Command())
	cmd.AddCommand((&CmdConnections{root: c}).Command())
	cmd.AddCommand((&CmdConnect{root: c}).Command())
	cmd.AddCommand((&CmdDisconnect{root: c}).Command())
	cmd.AddCommand((&CmdWarnings{root: c}).Command())
	cmd.AddCommand((&CmdOkay{root: c}).Command())
	cmd.AddCommand((&CmdSketch{root: c}).Command())
	cmd.AddCommand((&CmdSketches{root: c}).Command())
	cmd.AddCommand((&CmdDocs{root: c}).Command())

	cmd.PersistentFlags().StringVarP(&c.project, "project", "p", cwd, "Specify the project's directory path.")
	cmd.PersistentFlags().BoolP("help", "h", false, "Print the help message for the command.")

	cmd.DisableAutoGenTag = true

	return cmd
}

func (c *CmdRoot) client() (*client.Client, error) {
	if c.cli != nil {
		return c.cli, nil
	}

	cli, err := client.New(&ClientConfig)
	if err == nil {
		c.cli = cli
	} else {
		err = fmt.Errorf("cannot create client: %v", err)
	}

	return cli, err
}

func (c *CmdRoot) preRun(cmd *cobra.Command, args []string) error {
	project, err := filepath.Abs(c.project)
	if err != nil {
		return err
	}
	c.project = project
	return nil
}

func (c *CmdRoot) postRun(cmd *cobra.Command, args []string) {
	if c.cli != nil {
		maybePresentWarnings(c.cli.WarningsSummary())
	}
}

func (c *CmdRoot) completeWorkshopName(status []string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		return c.doCompleteWorkshopNames(args, status)
	}
}

func (c *CmdRoot) completeWorkshopNames(status []string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return c.doCompleteWorkshopNames(args, status)
	}
}

func (c *CmdRoot) doCompleteWorkshopNames(args []string, status []string) ([]string, cobra.ShellCompDirective) {
	cli, err := c.client()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	project, err := cli.Project(c.project)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	workshopInfo, _, err := cli.List(&client.ListOptions{ProjectId: project.Id})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	desiredStatus := func(s string) bool {
		if status == nil {
			return true
		}
		return slices.Contains(status, s)
	}

	var workshops []string
	for _, workshop := range workshopInfo {
		if desiredStatus(workshop.Status) && !slices.Contains(args, workshop.Name) {
			workshops = append(workshops, workshop.Name)
		}
	}
	return workshops, cobra.ShellCompDirectiveNoFileComp
}

var (
	// Standard streams, redirected for testing.
	Stdin  io.Reader = os.Stdin
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

// ClientConfig is the configuration of the Client used by all commands.
var ClientConfig = client.Config{
	// we need the powerful socket
	Socket: dirs.SocketPath,
}
