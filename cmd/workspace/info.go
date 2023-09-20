package main

import (
	"fmt"
	"strings"

	"github.com/canonical/workspace/client"
	"github.com/spf13/cobra"
)

type CmdInfo struct {
	waitMixin
}

func (c *CmdInfo) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "info <WORKSPACE>",
		Args:  cobra.RangeArgs(1, 1),
		Short: "Print the current status and details of a workspace as YAML.",
		Long: `
This command outputs the basic settings, current status and individual SDK
details for a workspace, formatting them as YAML. Specifically, it prints:

- Essential workspace attributes, such as name, base and project directory
- Current status (e.g. *Ready*, *Pending*, *Off*) and notes for the workspace
- Individual SDK details, such as name, channel, installation date and revision

Notes:
- Avoid assumptions based on SDK channels: 'latest/stable' may be neither
`,

		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdInfo) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create the client: %v", err)
	}

	c.setClient(cli)
	c.skipAbort = true

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	workspace, err := c.client.Workspace(project.Id, av[0])
	if err != nil {
		return err
	}

	w := tabWriter()

	fmt.Fprintf(w, "name:\t%s\n", workspace.Name)
	fmt.Fprintf(w, "base:\t%s\n", workspace.Base)
	fmt.Fprintf(w, "project:\t%s\n", project.Path)
	fmt.Fprintf(w, "status:\t%s\n", strings.ToLower(workspace.State))
	notes := strings.Join(workspace.Notes, ",")
	if len(workspace.Notes) == 0 {
		notes = "-"
	}
	fmt.Fprintf(w, "notes:\t%s\n", notes)

	if len(workspace.Content) > 0 {
		fmt.Fprintf(w, "content:\n")

		for _, sdk := range workspace.Content {
			fmt.Fprintf(w, "\t%s:\n", sdk.Name)
			installTime := sdk.InstallTime.Format("2006-01-02")
			if sdk.InstallTime.IsZero() {
				installTime = ""
			}
			fmt.Fprintf(w, "\t\tchannel:\t%s\t%s\t%s\n", sdk.Channel, installTime, sdk.Revision)
		}
	}

	w.Flush()

	return nil
}
