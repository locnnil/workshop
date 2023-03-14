package main

import (
	"errors"
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
	var err error
	var server srv.WorkspaceServer
	var project *workspace.Project
	var fs = afero.NewOsFs()

	/* check if both --project and --global were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("global").Changed {
		return fmt.Errorf("flags --project and --global are mutually exclusive")
	}

	server, err = srv.NewServer(fs)
	if err != nil {
		return err
	}

	if !c.global {
		project, err = workspace.LoadProject(server, fs, Project)

		if err == nil {
			/* List all workspaces for the current project */
			wsList, err := project.RetrieveWorkspaces()
			if len(wsList) != 0 && err == nil {
				listWorkspaces(wsList, project)
			} else {
				return err
			}
			return err
		} else if errors.Is(err, afero.ErrFileNotFound) {
			/* .lock file was not found in the current directory (or in its parents)
			   hence, we execute a global list command to view all the workspaces */
			listGlobal(server, fs)
		}
	} else {
		/* List all workspaces in all projects */
		err = listGlobal(server, fs)
		if err != nil {
			return err
		}
	}

	return nil
}

func listGlobal(server srv.WorkspaceServer, fs afero.Fs) error {
	wsList, err := workspace.RetrieveWorkspacesGlobal(server, fs)
	if err != nil || len(wsList) == 0 {
		return err
	}
	listAllWorkspaces(wsList)
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

	fmt.Fprintf(w, "Project\tWorkspace\tState\tNote\n")

	keys := maps.Keys(list)
	slices.SortFunc(keys,
		func(i, j *workspace.Project) bool { return i.ProjectDirectory() > j.ProjectDirectory() })

	for _, project := range keys {
		for _, j := range list[project] {
			comment := "-"
			if !project.Exists() {
				comment = "missing-project"
			}
			line := []string{
				contractHomeDirectory(project.ProjectDirectory()),
				j.Name,
				j.State.String(),
				comment,
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
		} else if filepath.HasPrefix(path, "(") {
			return "-"
		}
	}
	return path
}

func tabWriter() *tabwriter.Writer {
	/* Tab writer uses the same formatting as snap list */
	return tabwriter.NewWriter(os.Stdout, 5, 3, 2, ' ', 0)
}
