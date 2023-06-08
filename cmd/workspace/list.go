package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"text/tabwriter"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/dirs"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type CmdList struct {
	clientMixin
	global bool
}

func (c *CmdList) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(0),
		Short: "List workspaces",
		Long:  "The list command displays a summary of workspaces in the system",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.global, "global", false, "list workspaces from all projects")

	return cmd
}

func (c *CmdList) Run(cmd *cobra.Command, av []string) error {
	/* check if both --project and --global were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("global").Changed {
		return fmt.Errorf("flags --project and --global are mutually exclusive")
	}

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

	if !c.global {
		workspaces, err := c.client.ListWorkspaces(&client.ListOptions{ProjectId: project.Id})
		if err != nil {
			return err
		}
		/* List all workspaces for the current project */
		if len(workspaces) != 0 {
			listWorkspaces(workspaces, project)
		} else {
			return err
		}
		return err
	}

	return nil
}

func listWorkspaces(wsList []*client.Workspace, prj *client.ProjectResponse) {
	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\tState\tNotes\n")

	slices.SortFunc(wsList,
		func(i, j *client.Workspace) bool { return i.Name > j.Name })

	for _, val := range wsList {
		line := listWorkspace(val, prj)
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()
}

func listWorkspace(j *client.Workspace, prj *client.ProjectResponse) []string {
	comment := "-"
	if j.State == workspacebackend.Error.String() {
		comment = strings.Join(j.Notes, ",")
	}
	line := []string{
		contractHomeDirectory(prj.Path),
		j.Name,
		j.State,
		comment,
	}
	return line
}

/*
Make the path nicer and shorter by contracting $HOME with a ~

	TODO: Make it fully correct, filepath uses strings module which is not path-aware
*/
func contractHomeDirectory(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if filepath.HasPrefix(path, home) {
			return strings.Replace(path, home, "~", 1)
		} else if filepath.HasPrefix(path, "(") {
			return "-"
		}
	}
	return path
}

func tabWriter() *tabwriter.Writer {
	/* Tab writer uses the same formatting as snap list */
	return tabwriter.NewWriter(os.Stdout, 4, 3, 2, ' ', 0)
}
