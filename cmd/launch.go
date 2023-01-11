package main

import (
	"fmt"

	store "github.com/canonical/workspace/internal/fakestore"
	srv "github.com/canonical/workspace/internal/server"
	workspace "github.com/canonical/workspace/internal/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type CmdLaunch struct {
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "launch [workspace-name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Launch a workspace",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	fs := afero.NewOsFs()

	server, err := srv.NewServer(fs)
	if err != nil {
		fmt.Printf("%v", err)
		return err
	}

	ws, err := workspace.NewWorkspace(server, fs, ".workspace.project.yaml")
	if err != nil {
		fmt.Printf("%v", err)
		return err
	}

	storeClient, err := store.NewStoreClient(fs)
	if err != nil {
		return err
	}

	if err = ws.Launch(storeClient); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}

	return err
}
