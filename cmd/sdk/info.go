package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/canonical/lxd/shared/units"
	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/cmd/internal/cmdutil"
)

type CmdInfo struct {
	root *CmdRoot
}

func (c *CmdInfo) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <SDK>",
		Short: "Show SDK info",
		Long: `
This command prints the SDK's metadata
and lists workshops where the SDK is installed.

Notes:

- The output shows the SDK's build date.
- For an overview of SDK volumes, use 'sdk list'.
- For per-workshop information, use 'workshop info'.
`,
		Args: cobra.ExactArgs(1),
		RunE: c.Run,
	}
	return cmd
}

func (c *CmdInfo) Run(cmd *cobra.Command, av []string) error {
	cli, err := c.root.client()
	if err != nil {
		return err
	}

	info, err := cli.SdkInfo(av[0])
	if err != nil {
		return err
	}

	slices.SortFunc(info.Installed, func(a, b client.SdkInstalled) int {
		if a.Workshop != b.Workshop {
			return cmp.Compare(a.Workshop, b.Workshop)
		}
		return cmp.Compare(a.ProjectPath, b.ProjectPath)
	})

	fmt.Fprintf(Stdout, "name: %s\n", info.Name)
	fmt.Fprintf(Stdout, "summary: %s\n", cmdutil.EmptyDash(info.Summary))

	if info.Description != "" {
		fmt.Fprintln(Stdout, "description: |")
		lines := strings.Split(info.Description, "\n")
		for _, line := range lines {
			fmt.Fprintf(Stdout, "  %s\n", line)
		}
	} else {
		fmt.Fprintln(Stdout, "description: -")
	}

	fmt.Fprintln(Stdout, "installed:")
	w := tabwriter.NewWriter(Stdout, 4, 3, 2, ' ', 0)
	for _, it := range info.Installed {
		project := cmdutil.ContractHome(it.ProjectPath)
		channel := cmdutil.EmptyDash(it.Channel)
		date := formatDate(it.BuildTime)
		fmt.Fprintf(w, "  %s:\t%s\t%s\t%s\t(%s)\t%s\n",
			project, it.Workshop, channel, date, it.Revision, units.GetByteSizeString(int64(it.Size), 2))
	}
	w.Flush()

	return nil
}

func formatDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}
