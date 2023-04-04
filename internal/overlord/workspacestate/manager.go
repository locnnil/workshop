package workspacestate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/workspacebackend"
)

type WorkspaceManager struct {
	backend srv.WorkspaceBackend
}

func NewWorkspaceManager(runner *state.TaskRunner, server srv.WorkspaceBackend) *WorkspaceManager {
	manager := &WorkspaceManager{
		backend: server,
	}

	/* Workspace management */
	runner.AddHandler("create-workspace", manager.doCreateWorkspace, manager.undoCreateWorkspace)
	runner.AddHandler("start-workspace", manager.doStart, manager.undoStart)
	runner.AddHandler("mount-project", manager.doMountProject, manager.undoMountProject)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
