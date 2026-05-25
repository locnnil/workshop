// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/version"
)

const (
	GrpExplore = "explore-troubleshoot"
	GrpMisc    = "misc"
)

type CmdRoot struct {
	cli *client.Client
}

func (c *CmdRoot) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "sdk",
		SilenceErrors:              true,
		SilenceUsage:               true,
		TraverseChildren:           true,
		Version:                    version.Version,
		RunE:                       c.run,
		PersistentPostRun:          c.postRun,
		SuggestionsMinimumDistance: 2,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.DisableAutoGenTag = true

	groups := []*cobra.Group{{
		ID:    GrpExplore,
		Title: "Discover and inspect available SDKs:",
	}, {
		ID:    GrpMisc,
		Title: "Additional commands:",
	}}
	cmd.AddGroup(groups...)

	cmd.SetHelpCommandGroupID(GrpMisc)
	cmd.SetCompletionCommandGroupID(GrpMisc)

	cmd.AddCommand((&CmdFind{root: c}).Command())
	cmd.AddCommand((&CmdList{root: c}).Command())
	cmd.AddCommand((&CmdInfo{root: c}).Command())
	cmd.AddCommand((&CmdDocs{root: c}).Command())

	cmd.PersistentFlags().BoolP("help", "h", false, "Print the help message for the command.")
	cmd.PersistentFlags().BoolP("version", "v", false, "Print SDK CLI version.")

	return cmd
}

func (c *CmdRoot) run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	msg := fmt.Sprintf("unknown command %q", args[0])
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		msg += "\n\nDid you mean this?\n\t" + strings.Join(suggestions, "\n\t")
	}
	return errors.New(msg)
}

func (c *CmdRoot) client() (*client.Client, error) {
	if c.cli != nil {
		return c.cli, nil
	}

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create client: %w", err)
	}
	c.cli = cli
	return cli, nil
}

func (c *CmdRoot) postRun(cmd *cobra.Command, _ []string) {
	if c.cli != nil && cmd.Name() != cobra.ShellCompRequestCmd {
		c.cli.CloseIdleConnections()
	}
}

var (
	Stdin  io.Reader = os.Stdin
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

var ClientConfig = client.Config{
	Socket: dirs.SocketPath,
}
