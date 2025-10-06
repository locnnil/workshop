package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/version"
)

type CmdRoot struct {
	cli *client.Client
}

func (c *CmdRoot) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "sdk",
		Short:             "Inspect SDK volumes installed on the system",
		SilenceErrors:     true,
		SilenceUsage:      true,
		TraverseChildren:  true,
		Version:           version.Version,
		PersistentPostRun: c.postRun,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.DisableAutoGenTag = true

	cmd.AddCommand((&CmdList{root: c}).Command())

	cmd.PersistentFlags().BoolP("help", "h", false, "Print the help message for the command.")
	cmd.PersistentFlags().BoolP("version", "v", false, "Print SDK CLI version.")

	return cmd
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
