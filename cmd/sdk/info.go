package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
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

	fmt.Fprintf(Stdout, "name: %s\n", info.Name)
	fmt.Fprintf(Stdout, "summary: %s\n", emptyDash(info.Summary))

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
		project := contractHome(it.ProjectPath)
		channel := emptyDash(it.Channel)
		date := formatDate(it.BuildTime)
		fmt.Fprintf(w, "  %s:\t%s\t%s\t%s\t(%s)\t%s\n",
			project, it.Workshop, channel, date, it.Revision, formatSize(it.Size))
	}
	w.Flush()

	return nil
}

func contractHome(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if path == home || strings.HasPrefix(path, home+"/") {
			return strings.Replace(path, home, "~", 1)
		} else if strings.HasPrefix(path, "(") {
			return "-"
		}
	}
	return path
}

func formatDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}

func formatSize(sz uint64) string {
	if sz == 0 {
		return "-"
	}
	const kb = 1024
	const mb = 1024 * 1024
	if sz < mb {
		return fmt.Sprintf("%dKB", sz/kb)
	}
	return fmt.Sprintf("%dMB", sz/mb)
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
