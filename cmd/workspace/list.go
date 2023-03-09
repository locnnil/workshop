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
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
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

	if !c.all {
		project, err := workspace.LoadProject(server, fs, Project)
		if err == workspace.ErrProjectFileNotFound {
			project, err = workspace.NewProject(server, fs, Project)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		/* List all workspaces for the current project */
		wsList, err := project.EnumWorkspaces()
		if len(wsList) != 0 && err == nil {
			listWorkspaces(wsList, project)
		} else {
			return err
		}

	} else {
		/* List all workspaces in all projects */
		wsList, err := workspace.EnumAllWorkspaces(server, fs)
		if err != nil || len(wsList) == 0 {
			return err
		}
		listAllWorkspaces(wsList)
	}

	return nil
}

func listWorkspaces(wsList []*srv.WorkspaceProps, project *workspace.Project) {
	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\tState\n")

	for _, val := range wsList {
		line := []string{
			contractHomeDirectory(project.ProjectDirectory()),
			val.Name,
			val.State.String(),
		}
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()
}

func listAllWorkspaces(list map[*workspace.Project][]*srv.WorkspaceProps) {
	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\tState\n")

	keys := maps.Keys(list)
	slices.SortFunc(keys,
		func(i, j *workspace.Project) bool { return i.ProjectDirectory() > j.ProjectDirectory() })

	for _, project := range keys {
		for _, j := range list[project] {
			line := []string{
				contractHomeDirectory(project.ProjectDirectory()),
				j.Name,
				j.State.String(),
			}
			fmt.Fprintln(w, strings.Join(line, "\t"))
		}
	}
	w.Flush()
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
