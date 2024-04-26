package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
)

type clientMixin struct {
	cli *client.Client
}

func (ch *clientMixin) setClient(cli *client.Client) {
	ch.cli = cli
}

func (ch *clientMixin) client() *client.Client {
	return ch.cli
}

var rootCmd = &cobra.Command{
	Use:              "workshop",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var (
	// Standard streams, redirected for testing.
	Stdin  io.Reader = os.Stdin
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)
var Project string

// ClientConfig is the configuration of the Client used by all commands.
var ClientConfig = client.Config{
	// we need the powerful socket
	Socket: dirs.SocketPath,
}

func postRunWarnings(c *clientMixin) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		if c.client() != nil {
			maybePresentWarnings(c.client().WarningsSummary())
		}
	}
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	l, err := logger.New(os.Stderr, 0)
	if err != nil {
		panic(err)
	}

	logger.SetLogger(l)

	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", cwd, "Specify the project's directory path.")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print the help message for the command.")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
	rootCmd.AddCommand((&CmdList{}).Command())
	rootCmd.AddCommand((&CmdChanges{}).Command())
	rootCmd.AddCommand((&CmdTasks{}).Command())
	rootCmd.AddCommand((&CmdRefresh{}).Command())
	rootCmd.AddCommand((&CmdStart{}).Command())
	rootCmd.AddCommand((&CmdStop{}).Command())
	rootCmd.AddCommand((&CmdInfo{}).Command())
	rootCmd.AddCommand((&CmdExec{}).Command())
	rootCmd.AddCommand((&CmdShellAlias{}).Command())
	rootCmd.AddCommand((&CmdRemove{}).Command())
	rootCmd.AddCommand((&CmdRemount{}).Command())
	rootCmd.AddCommand((&CmdConnections{}).Command())
	rootCmd.AddCommand((&CmdConnect{}).Command())
	rootCmd.AddCommand((&CmdDisconnect{}).Command())
	rootCmd.AddCommand((&CmdWarnings{}).Command())
	rootCmd.AddCommand((&CmdOkay{}).Command())

	rootCmd.SilenceErrors = true

	if err = rootCmd.Execute(); err != nil {
		fmt.Fprintf(Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
