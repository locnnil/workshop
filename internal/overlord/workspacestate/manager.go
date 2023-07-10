package workspacestate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/sthelper"
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
	AddHandler(runner, "create-workspace", manager.doCreateWorkspace, manager.undoCreateWorkspace, WaitOnErrorDecorator)
	AddHandler(runner, "start-workspace", manager.doStart, manager.doStop, WaitOnErrorDecorator)
	AddHandler(runner, "stop-workspace", manager.doStop, manager.doStart, WaitOnErrorDecorator)
	AddHandler(runner, "delete-workspace", manager.doDeleteWorkspace, nil, WaitOnErrorDecorator)
	AddHandler(runner, "mount-project", manager.doMountProject, manager.undoMountProject, WaitOnErrorDecorator)
	AddHandler(runner, "complete-refresh", manager.doCompleteRefresh, nil, WaitOnErrorDecorator)
	AddHandler(runner, "start-refresh", manager.doStartRefresh, manager.undoStartRefresh, WaitOnErrorDecorator)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
