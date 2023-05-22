package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"text/tabwriter"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	srv "github.com/canonical/workspace/internal/workspacebackend"
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
	var server srv.WorkspaceBackend
	var project *projectstate.Project
	var fs = afero.NewOsFs()

	/* check if both --project and --global were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("global").Changed {
		return fmt.Errorf("flags --project and --global are mutually exclusive")
	}

	server, err = srv.New()
	if err != nil {
		return err
	}

	if !c.global {
		project, err = projectstate.LoadProject(server, fs, Project)
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
			/* Project was not found at the path provided, hence
			return an error */
			return fmt.Errorf("not a project directory. Try --global to see all projects or launch your first workspace")
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

func listGlobal(server srv.WorkspaceBackend, fs afero.Fs) error {
	list, err := projectstate.RetrieveWorkspacesGlobal(server, fs)
	if err != nil || len(list) == 0 {
		return err
	}
	w := tabWriter()

	fmt.Fprintf(w, "Project\tWorkspace\tState\tNotes\n")

	keys := maps.Keys(list)
	slices.SortFunc(keys,
		func(i, j *projectstate.Project) bool { return i.ProjectDirectory() > j.ProjectDirectory() })

	for _, project := range keys {
		for _, j := range list[project] {
			if j.State() == util.Inactive {
				continue
			}
			line := listWorkspace(j, project)
			fmt.Fprintln(w, strings.Join(line, "\t"))
		}
	}
	w.Flush()
	return nil
}

func listWorkspaces(wsList []*srv.WorkspaceProps, project *projectstate.Project) {
	/* if all workspaces are inactive, we do not list them */
	isAllInactive := func(i *srv.WorkspaceProps) bool { return i.State() != util.Inactive }
	if slices.IndexFunc(wsList, isAllInactive) == -1 {
		return
	}

	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\tState\tNotes\n")

	slices.SortFunc(wsList,
		func(i, j *srv.WorkspaceProps) bool { return i.Name > j.Name })

	for _, val := range wsList {
		if val.State() == util.Inactive {
			continue
		}
		line := listWorkspace(val, project)
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()
}

func listWorkspace(j *srv.WorkspaceProps, project *projectstate.Project) []string {
	comment := "-"
	if j.State() == util.Error {
		comment = j.Reason().String()
	}
	line := []string{
		contractHomeDirectory(project.ProjectDirectory()),
		j.Name,
		j.State().String(),
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
	return tabwriter.NewWriter(os.Stdout, 5, 3, 2, ' ', 0)
}
