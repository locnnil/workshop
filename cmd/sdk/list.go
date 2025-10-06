package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type CmdList struct {
	root *CmdRoot
}

func (c *CmdList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SDK volumes available on this machine",
		Long: `
This command lists all local SDK volumes.

Use it to enumerate the SDK revisions currently stored on the system.
Only volumes are reported, not the workshops that use them.

Notes:

- For per-workshop information, use 'workshop info'.
- Multiple entries may appear for a single SDK
  if several revisions are present simultaneously.
`,
		Args: cobra.NoArgs,
		RunE: c.Run,
	}

	return cmd
}

func (c *CmdList) Run(cmd *cobra.Command, _ []string) error {
	cli, err := c.root.client()
	if err != nil {
		return err
	}

	sdks, err := cli.Sdks()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(Stdout, 4, 3, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tVersion\tRev\tSummary\n")
	for _, sdk := range sdks {
		version := sdk.Version
		if version == "" {
			version = "-"
		}

		summary := sdk.Summary
		if summary == "" {
			summary = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sdk.Name, version, sdk.Revision, summary)
	}

	return w.Flush()
}
