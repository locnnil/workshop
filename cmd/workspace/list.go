package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"text/tabwriter"

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

	cmd.Flags().BoolVar(&c.all, "all", true, "list workspaces from all projects")

	return cmd
}

func (c *CmdList) Run(cmd *cobra.Command, av []string) error {

	/* check if both --project and --all were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("all").Changed {
		return fmt.Errorf("flags --project and --all are mutually exclusive")
	}

	fs := afero.NewOsFs()

	wsList, err := workspace.EnumWorkspaces(fs, Project)
	if err != nil || len(wsList) == 0 {
		return err
	}

	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\n")

	for i := range wsList {
		line := []string{
			contractHomeDirectory(Project),
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
