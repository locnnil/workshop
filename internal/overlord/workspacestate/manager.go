package workspacestate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"
)

type WorkspaceManager struct {
	backend workspacebackend.WorkspaceBackend
}

func NewWorkspaceManager(runner *state.TaskRunner, server workspacebackend.WorkspaceBackend) *WorkspaceManager {
	manager := &WorkspaceManager{
		backend: server,
	}

	/* Workspace management */
	AddHandler(runner, "create-workspace", manager.doCreateWorkspace, manager.undoCreateWorkspace, WaitOnErrorDecorator)
	AddHandler(runner, "start-workspace", manager.doStart, manager.doStop, WaitOnErrorDecorator)
	AddHandler(runner, "stop-workspace", manager.doStop, manager.doStart, WaitOnErrorDecorator)
	AddHandler(runner, "delete-workspace", manager.doDeleteWorkspace, nil, WaitOnErrorDecorator)
	AddHandler(runner, "mount-project", manager.doMountProject, manager.undoMountProject, WaitOnErrorDecorator)
	AddHandler(runner, "delete-refresh-backup", manager.doDeleteRefreshBackup, nil, WaitOnErrorDecorator)
	AddHandler(runner, "make-refresh-backup", manager.doMakeRefreshBackup, manager.undoMakeRefreshBackup, WaitOnErrorDecorator)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
