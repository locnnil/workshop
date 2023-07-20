package main

import (
	"io"
	"os"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/dirs"
	"github.com/canonical/workspace/internal/logger"

	"github.com/spf13/cobra"
)

type clientSetter interface {
	setClient(*client.Client)
}

type clientMixin struct {
	client *client.Client
}

func (ch *clientMixin) setClient(cli *client.Client) {
	ch.client = cli
}

var rootCmd = &cobra.Command{
	Use:              "workspace",
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
	// we need the powerful snapd socket
	Socket: dirs.WorkspaceSocket,
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

	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", cwd, "specify a project's directory path")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
	rootCmd.AddCommand((&CmdList{}).Command())
	rootCmd.AddCommand((&CmdChanges{}).Command())
	rootCmd.AddCommand((&CmdTasks{}).Command())
	rootCmd.AddCommand((&CmdRefresh{}).Command())

	if err = rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
