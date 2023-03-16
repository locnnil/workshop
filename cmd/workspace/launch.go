package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	srv "github.com/canonical/workspace/internal/server"
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
	if errors.Is(err, afero.ErrFileNotFound) {
		project, err = workspace.NewProject(server, fs, Project)
		if err != nil {
			return err
		}
		project.SaveProject()
	} else if err != nil {
		return err
	}

	wsList, err := project.EnumWorkspaceFiles()
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
			fmt.Printf("\nUse \"workspace launch \033[3mname\033[0m\" to disambiguate\n")
			return nil
		}
	}

	/* If the name was provided by the user, test if we have such a workspace */
	finder := func(p *srv.WorkspaceProps) bool { return p.Name == wsName }
	idx := slices.IndexFunc(wsList, finder)
	if idx == -1 {
		return fmt.Errorf("workspace \"\033[1m%s\033[0m\" not found", util.ToFileName(wsName))
	}

	file, err := workspace.ReadWorkspace(project, wsName)
	if err != nil {
		return err
	}

	overlord, err := overlord.New(server, nil, os.Stdout)
	if err != nil {
		return err
	}

	overlord.Loop()

	st := overlord.State()
	st.Lock()

	taskset, err := workspace.Launch(st, project, file)
	if err != nil {
		return err
	}
	change := st.NewChange("launch", fmt.Sprintf("Launch workspace %q", wsName))
	projectKey := workspace.ProjectKey{
		Path:      project.ProjectDirectory(),
		ProjectId: project.ProjectId(),
	}

	change.Set("project-key", projectKey)
	change.Set("workspace", wsName)

	change.AddAll(taskset)
	st.EnsureBefore(0)
	st.Unlock()

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
out:
	for {
		select {
		case sig := <-sigs:
			logger.Noticef("Exiting on %s signal.\n", sig)
			break out
		case <-change.Ready():
			fmt.Printf("Workspace \"%s\" started.\n", wsName)
			break out
		}
	}

	if change.Status().Ready() {
		if change.Err() != nil {
			fmt.Print(change.Err())
		}
	}

	return overlord.Stop()
}

func printWorkspaces(wsList []*srv.WorkspaceProps) {
	if len(wsList) > 0 {
		fmt.Printf("Available workspaces:\n")
		for _, k := range wsList {
			fmt.Printf("  \033[1m%s\033[0m\n", k.Name)
		}
	}
}
