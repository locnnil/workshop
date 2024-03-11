package main

import (
	"fmt"
	"strings"

	"github.com/canonical/workshop/client"
	"github.com/spf13/cobra"
)

type CmdInfo struct {
	waitMixin
}

func (c *CmdInfo) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "info <WORKSHOP>",
		Args:  cobra.RangeArgs(1, 1),
		Short: "Print the current status and details of a workshop as YAML.",
		Long: `
This command outputs the basic settings, current status and individual SDK
details for a workshop, formatting them as YAML. Specifically, it prints:

- Essential workshop attributes, such as name, base and project directory
- Current status (e.g. *Ready*, *Pending*, *Off*) and notes for the workshop
- Individual SDK details, such as name, channel, installation date and revision

Notes:
- Avoid assumptions based on SDK channels: 'latest/stable' may be neither
`,

		RunE: c.Run,
	}

	return cmd
}

func (c *CmdInfo) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)
	c.skipAbort = true

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	workshop, err := c.client.Workshop(project.Id, av[0])
	if err != nil {
		return err
	}

	w := tabWriter()

	fmt.Fprintf(w, "name:\t%s\n", workshop.Name)
	fmt.Fprintf(w, "base:\t%s\n", workshop.Base)
	fmt.Fprintf(w, "project:\t%s\n", project.Path)
	fmt.Fprintf(w, "status:\t%s\n", strings.ToLower(workshop.Status))

	// get the workshop notes
	notes := workshop.Notes

	// get the SDKs notes (if there is an ongoing health check)
	for _, sdk := range workshop.Content {
		if sdk.Health != nil && sdk.Health.Code != "" {
			notes = append(notes, sdk.Health.Code)
		}
	}

	// combine notes from workshop and its SDKs
	notesFormatted := strings.Join(notes, ",")
	if len(workshop.Notes) == 0 {
		notesFormatted = "-"
	}

	fmt.Fprintf(w, "notes:\t%s\n", notesFormatted)

	if len(workshop.Content) > 0 {
		fmt.Fprintf(w, "content:\n")
		for _, sdk := range workshop.Content {
			fmt.Fprintf(w, "  %s:\n", sdk.Name)
			installTime := sdk.InstallTime.Format("2006-01-02")
			if sdk.InstallTime.IsZero() {
				installTime = ""
			}
			fmt.Fprintf(w, "    channel:\t%s\t%s\t%s\n", sdk.Channel, installTime, sdk.Revision)
			if sdk.Health != nil {
				fmt.Fprintf(w, "    message:\t%s\n", sdk.Health.Message)
			}

			if len(sdk.Mounts) > 0 {
				fmt.Fprintf(w, "    mounts:\n")
				for _, mount := range sdk.Mounts {
					fmt.Fprintf(w, "      %s:\n", mount.Plug.Name)
					fmt.Fprintf(w, "        host:\t%s\n", mount.Source)
					fmt.Fprintf(w, "        workshop:\t%s\n", mount.Target)
				}
			}
		}
	}

	w.Flush()

	return nil
}
