package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"text/tabwriter"

	srv "github.com/canonical/workspace/internal/server"
	workspace "github.com/canonical/workspace/internal/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type CmdList struct {
	all bool
}

func (c *CmdList) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "list",
		Args:  cobra.MaximumNArgs(0),
		Short: "List workspaces",
		Long:  "The list command displays a summary of workspaces in the system",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.all, "all", false, "list workspaces from all projects")

	return cmd
}

func (c *CmdList) Run(cmd *cobra.Command, av []string) error {
	var wsList map[string]srv.WorkspaceProps
	var err error
	var server srv.WorkspaceServer
	var fs = afero.NewOsFs()

	/* check if both --project and --all were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("all").Changed {
		return fmt.Errorf("flags --project and --all are mutually exclusive")
	}

	server, err = srv.NewServer(fs)
	if err != nil {
		return err
	}

	project, err := workspace.NewProject(server, fs, Project)
	if err != nil {
		return err
	}

	if !c.all {
		/* List all workspaces for the current project */
		wsList, err = project.EnumWorkspaces()
	} else {
		/* List all workspaces in all projects */
		/* This is a naive approach that works with all the workspaces by
		   enumerating them and their project mounts. It does not handle cases
		   when a directory of the project was (re)moved, for example. To be substituted
		   with a more decent implementation in the next iteration */
		wsList, err = project.EnumAllWorkspaces()
	}

	if err != nil || len(wsList) == 0 {
		return err
	}

	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\tState\n")

	for i, val := range wsList {
		line := []string{
			contractHomeDirectory(project.GetProjectDirectory()),
			i,
			val.State.String(),
		}
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()

	return nil
}

/*
Make the path nicer and shorter by contracting $HOME with a ~

	TODO: Make it fully correct, filepath uses strings module which is not path-aware
*/
func contractHomeDirectory(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if filepath.HasPrefix(path, home) {
			return strings.Replace(path, home, "~", 1)
		}
	}
	return path
}

func tabWriter() *tabwriter.Writer {
	/* Tab writer uses the same formatting as snap list */
	return tabwriter.NewWriter(os.Stdout, 5, 3, 2, ' ', 0)
}
