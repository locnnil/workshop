package main

import (
	"fmt"
	"os"

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
	var wsName string
	fs := afero.NewOsFs()

	if len(av) == 1 {
		wsName = av[0]
	}

	wsList, err := workspace.EnumWorkspaces(fs, Project)
	if err != nil || len(wsList) == 0 && wsName == "" {
		fmt.Printf("Could not find a workspace to launch. Start by creating a .workspace.<name>.yaml for the project.\n")
		return err
	} else if len(wsList) > 1 && wsName == "" {
		printWorkspaces(wsList)
		fmt.Printf("\nUse \"workspace launch \033[3mname\033[0m\" to disambiguate.\n")
		return nil
	} else if len(wsList) == 1 && wsName == "" {
		/* Handle the case with a single workspace found but no args provided to the command,
		   it's a map, so iterate to the first element */
		for i := range wsList {
			wsName = i
			break
		}
	}

	/* If the name was provided by the user, test if we have such a workspace */
	if _, ok := wsList[wsName]; !ok {
		fmt.Printf("\033[1m%s\033[0m not found.\n", wsName)
		printWorkspaces(wsList)
		os.Exit(1)
	}

	server, err := srv.NewServer(fs)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	ws, err := workspace.NewWorkspace(server, fs, wsList[wsName])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	storeClient, err := store.NewStoreClient(fs)
	if err != nil {
		os.Exit(1)
	}

	if err = ws.Launch(storeClient); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	return err
}

func printWorkspaces(wsList map[string]workspace.WorkspaceFile) {
	if len(wsList) > 0 {
		fmt.Printf("Available workspaces:\n")
		for _, k := range wsList {
			fmt.Printf("  \033[1m%s\033[0m\n", k.Name)
		}
	}
}
