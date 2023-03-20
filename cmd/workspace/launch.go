package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	srv "github.com/canonical/workspace/internal/server"

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

	server, err := srv.NewServer(fs)
	if err != nil {
		return err
	}

	file, err := workspace.ReadWorkspace(fs, util.ToPathname(Project, ws))
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

	task, err := projectstate.LoadOrCreate(st, Project)
	if err != nil {
		return err
	}

	taskset, err := workspace.Launch(st, file)
	if err != nil {
		return err
	}

	/* a project must be loaded before doing anything else */

	taskset.WaitFor(task)

	change := st.NewChange("launch", fmt.Sprintf("Launch workspace %q", ws))
	change.Set("workspace", ws)

	change.AddTask(task)
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
			change.Abort()
			break out
		case <-change.Ready():
			fmt.Printf("Workspace \"%s\" started.\n", ws)
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
