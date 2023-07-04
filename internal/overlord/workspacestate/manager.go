package workspacestate

import (
	"errors"

	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/tomb.v2"
)

type WorkspaceManager struct {
	backend srv.WorkspaceBackend
}

func KeepOnErrorDecorator(f state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		return f(task, tomb)
	}
}

func NewWorkspaceManager(runner *state.TaskRunner, server srv.WorkspaceBackend) *WorkspaceManager {
	manager := &WorkspaceManager{
		backend: server,
	}

	/* Workspace management */
	runner.AddHandler("create-workspace", KeepOnErrorDecorator(manager.doCreateWorkspace), manager.undoCreateWorkspace)
	runner.AddHandler("start-workspace", manager.doStart, manager.doStop)
	runner.AddHandler("stop-workspace", manager.doStop, manager.doStart)

	runner.AddHandler("mount-project", manager.doMountProject, manager.undoMountProject)
	runner.AddHandler("delete-unavailable-workspace", manager.doDeleteUnavailableWorkspace, nil)
	runner.AddHandler("make-unavailable", manager.doMakeUnavailable, manager.doMakeAvailable)
	runner.AddHandler("make-available", manager.doMakeAvailable, manager.doMakeUnavailable)

	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return errors.New("error")
	}
	runner.AddHandler("error-trigger", erroringHandler, nil)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
