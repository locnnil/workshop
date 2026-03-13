package main

import (
	"cmp"
	"fmt"
	"slices"
	"text/tabwriter"

	"github.com/canonical/lxd/shared/units"
	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
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

- For per-workshop information, use "workshop info".
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

	if len(sdks) == 0 {
		return nil
	}

	slices.SortFunc(sdks, func(a, b client.SdkVolume) int {
		return cmp.Compare(a.Name, b.Name)
	})

	w := tabwriter.NewWriter(Stdout, 4, 3, 2, ' ', tabwriter.StripEscape)
	maxRev := len("REV")
	maxSize := len("SIZE")
	for _, sdk := range sdks {
		maxRev = max(maxRev, len(sdk.Revision))
		maxSize = max(maxSize, len(units.GetByteSizeString(sdk.Size, 2)))
	}
	fmt.Fprintf(w, "NAME\tVERSION\t%*s\t%*s\n", maxRev, "REV", maxSize, "SIZE")
	for _, sdk := range sdks {
		version := sdk.Version
		if version == "" {
			version = "-"
		}

		size := units.GetByteSizeString(sdk.Size, 2)
		fmt.Fprintf(w, "%s\t%s\t%*s\t%*s\n", sdk.Name, version, maxRev, sdk.Revision, maxSize, size)
	}

	return w.Flush()
}
