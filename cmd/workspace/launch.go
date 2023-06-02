package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"syscall"

	"github.com/canonical/workspace/internal/dirs"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/state"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type CmdLaunch struct {
}

func (c *CmdLaunch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "launch workspace-name",
		Args:  cobra.MinimumNArgs(1),
		Short: "Launch a workspace",
		RunE:  c.Run,
	}

	return cmd
}

func (c *CmdLaunch) Run(cmd *cobra.Command, av []string) error {
	var ws string
	fs := afero.NewOsFs()

	ws = av[0]

	file, err := workspacebackend.ReadWorkspace(fs, workspacebackend.WorkspaceFilePath(Project, ws))
	if err != nil {
		return err
	}

	workspaceDir, _ := dirs.GetEnvPaths()

	overlord, err := overlord.New(workspaceDir, nil)
	if err != nil {
		return err
	}

	overlord.Loop()

	st := overlord.State()
	st.Lock()

	username, err := user.Current()
	if err != nil {
		return err
	}

	userCtx := context.WithValue(context.Background(), workspacebackend.ContextUser, username.Username)

	projectKey, err := project.RetrieveProject(userCtx, overlord.WorkspaceBackend(), fs, Project)
	if err != nil {
		return err
	}

	taskset, err := workspace.Launch(st, file)
	if err != nil {
		return err
	}

	change := st.NewChange("launch", fmt.Sprintf("Launch workspace %q", ws))
	change.Set("workspace", ws)
	change.Set("project-key", projectKey)
	change.Set("user", username.Username)

	change.AddAll(taskset)
	st.Unlock()

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-sigs
		logger.Debugf("Exiting on %s signal.\n", sig)
		st.Lock()
		change.Abort()
		st.EnsureBefore(0)
		st.Unlock()
	}()

	<-change.Ready()

	st.Lock()
	if change.Status().Ready() {
		launched := true
		for _, t := range change.Tasks() {
			if t.Status() != state.DoneStatus {
				launched = false
			}
		}
		if change.Err() != nil {
			fmt.Print(change.Err())
		} else if change.Status() == state.UndoneStatus {
			fmt.Println("Aborted.")
		} else if launched {
			fmt.Printf("Workspace \"%s\" started.\n", ws)
		}
	}
	st.Unlock()

	return overlord.Stop()
}
