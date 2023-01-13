package main

import (
	"fmt"
	"os"
	"regexp"

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

func enumWorkspaces(fsys afero.Fs) (map[string]workspace.WorkspaceFile, error) {
	var workspaces = make(map[string]workspace.WorkspaceFile, 0)
	var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z0-9]+)\.yaml$`)

	files, err := afero.ReadDir(fsys, ".")
	if err != nil {
		return workspaces, nil
	}

	for _, info := range files {
		if info.IsDir() {
			continue
		}

		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); len(names) > 0 {
			workspaces[names[1]] = workspace.WorkspaceFile{Name: names[1], File: info}
		}
	}
	return workspaces, nil
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	var wsName string
	fs := afero.NewOsFs()

	if len(av) == 1 {
		wsName = av[0]
	}

	wsList, err := enumWorkspaces(fs)
	if err != nil || len(wsList) == 0 {
		fmt.Printf("Could not find a workspace to launch. Start with creating a .workspace.<name>.yaml for the project.")
		return err
	} else if len(wsList) > 1 && wsName == "" {
		printWorkspaces(wsList)
		fmt.Printf("\nUse \"workspace launch \033[3mname\033[0m\" to disambiguate.\n")
		return nil
	} else if len(wsList) == 1 && wsName == "" {
		/* Handle the case with a single workspace found but no args provided to the command */
		for i := range wsList {
			wsName = i
			break
		}
	}

	if _, ok := wsList[wsName]; !ok {
		fmt.Printf("\033[1m%s\033[0m not found. ", wsName)
		printWorkspaces(wsList)
		return os.ErrNotExist
	}

	server, err := srv.NewServer(fs)
	if err != nil {
		fmt.Printf("%v", err)
		return err
	}

	ws, err := workspace.NewWorkspace(server, fs, wsList[wsName])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
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

func printWorkspaces(wsList map[string]workspace.WorkspaceFile) {
	fmt.Printf("Available workspaces:\n")
	for _, k := range wsList {
		fmt.Printf("  \033[1m%s\033[0m\n", k.Name)
	}
}
