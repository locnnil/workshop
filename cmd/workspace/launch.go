package main

import (
	"fmt"
	"os"

	store "github.com/canonical/workspace/internal/fakestore"
	srv "github.com/canonical/workspace/internal/server"
	workspace "github.com/canonical/workspace/internal/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
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

	server, err := srv.NewServer(fs)
	if err != nil {
		return err
	}

	project, err := workspace.LoadProject(server, fs, Project)
	if err == workspace.ErrProjectFileNotFound {
		project, err = workspace.NewProject(server, fs, Project)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	wsList, err := project.EnumWorkspaces()
	if err != nil || len(wsList) == 0 {
		return err
	}

	/* if no name provided, try to see if we can disambiguate */
	if wsName == "" {
		if len(wsList) == 1 {
			/* If no names provided and there is only one workspace - run it */
			wsName = wsList[0].Name
		} else if len(wsList) > 1 {
			/* If there are multiple workspaces and no names provided - ask a user to resolve */
			printWorkspaces(wsList)
			fmt.Printf("\nUse \"workspace launch \033[3mname\033[0m\" to disambiguate.\n")
			return nil
		}
	}

	finder := func(p *srv.WorkspaceProps) bool { return p.Name == wsName }
	idx := slices.IndexFunc(wsList, finder)

	/* If the name was provided by the user, test if we have such a workspace */
	if idx == -1 {
		fmt.Printf("workspace \"\033[1m%s\033[0m\" not found.\n", wsName)
		printWorkspaces(wsList)
		os.Exit(1)
	}

	ws, err := workspace.NewWorkspace(server, project, fs, wsList[idx])
	if err != nil {
		return err
	}

	storeClient, err := store.NewStoreClient(fs)
	if err != nil {
		return err
	}

	/* We are officially launching here, so whatever happens, the project should persist if still not */
	defer project.SaveProject()

	if err = ws.Launch(storeClient); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	return err
}

func printWorkspaces(wsList []*srv.WorkspaceProps) {
	if len(wsList) > 0 {
		fmt.Printf("Available workspaces:\n")
		for _, k := range wsList {
			fmt.Printf("  \033[1m%s\033[0m\n", k.Name)
		}
	}
}
