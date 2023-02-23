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

	/* check if both --project and --all were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("all").Changed {
		return fmt.Errorf("flags --project and --all are mutually exclusive")
	}

	fs := afero.NewOsFs()

	var wsList map[string]srv.WorkspaceFile
	var err error
	if !c.all {
		/* List all workspaces for the current project */
		wsList, err = workspace.EnumWorkspaces(fs, Project)
	} else {
		/* List all workspaces in all projects */
		/* This is a naive approach that works with all the workspaces by
		   enumerating them and their project mounts. It does not handle cases
		   when a directory of the project was (re)moved, for example. To be substituted
		   with a more decent implementation in the next iteration */
		var server srv.WorkspaceServer
		server, err = srv.NewServer(fs)
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		wsList, err = workspace.EnumAllWorkspaces(server)
	}

	if err != nil || len(wsList) == 0 {
		return err
	}

	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\n")

	for i, k := range wsList {
		line := []string{
			contractHomeDirectory(k.ProjectPath),
			i,
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
