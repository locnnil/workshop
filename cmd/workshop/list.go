package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"text/tabwriter"

	"github.com/canonical/workshop/client"
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
		Short: "List project workshops.",
		Long: `
This command enumerates all workshops in the project, printing a compact list:

- Project: absolute pathname of the project where this workshop belongs
- Workshop: workshop name, as set by its definition
- State: workshop status, such as *Off*, *Ready*, *Pending* and so on
- Notes: internal remarks on the overall state of the workshop

The '--global' option lists all workshops from *all* projects in the system;
however, it doesn't include any that are *Off*.

Notes:
- For details of a single workshop, use 'workshop info' instead
`,
		RunE: c.Run,
	}

	cmd.Flags().BoolVar(&c.global, "global", false, "List workshops from all projects in the system")

	return cmd
}

func (c *CmdList) Run(cmd *cobra.Command, av []string) error {
	/* check if both --project and --global were provided */
	if cmd.Parent().Flag("project").Changed && cmd.Flag("global").Changed {
		return fmt.Errorf("cannot list: '--project' incompatible with '--global'")
	}

	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	if !c.global {
		project, err := c.client.Project(Project)
		if err != nil {
			return err
		}

		workshops, err := c.client.ListWorkspaces(&client.ListOptions{ProjectId: project.Id})
		if err != nil {
			return err
		}
		slices.SortFunc(workshops, func(a, b *client.Workshop) bool { return a.Name < b.Name })
		/* List all workshops for the current project */
		if len(workshops) != 0 {
			printWorkspaces(workshops, project)
		} else {
			return err
		}
		return err
	} else {
		w := tabWriter()
		fmt.Fprintf(w, "Project\tWorkspace\tState\tNotes\n")

		projects, err := c.client.Projects()
		slices.SortFunc(projects, func(a, b *client.Project) bool { return a.Path < b.Path })

		if err != nil {
			return err
		}

		for _, i := range projects {
			workshops, err := c.client.ListWorkspaces(&client.ListOptions{ProjectId: i.Id})
			slices.SortFunc(workshops, func(a, b *client.Workshop) bool { return a.Name < b.Name })

			if err != nil {
				return err
			}
			for _, j := range workshops {
				// --global flag would not list Off workshops for consistency.
				// We may not be aware of all the project directories on the system
				// and, thus, will not know all the available Off workshops (contrary
				// to the workshops that are in any other state, i.e. running instances, which we always know
				// about from the workshop backend)
				if j.State != "Off" {
					fmt.Fprintln(w, strings.Join(printWorkspace(j, i), "\t"))
				}
			}
		}

		w.Flush()
	}

	return nil
}

func printWorkspaces(wsList []*client.Workshop, prj *client.Project) {
	w := tabWriter()
	fmt.Fprintf(w, "Project\tWorkspace\tState\tNotes\n")

	for _, val := range wsList {
		line := printWorkspace(val, prj)
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()
}

func printWorkspace(j *client.Workshop, prj *client.Project) []string {
	comment := "-"
	if len(j.Notes) > 0 {
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
	return tabwriter.NewWriter(Stdout, 4, 3, 2, ' ', 0)
}
