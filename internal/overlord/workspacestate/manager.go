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
	runner.AddHandler("create-workspace", OnDoError(manager.doCreateWorkspace), manager.undoCreateWorkspace)
	runner.AddHandler("start-workspace", OnDoError(manager.doStart), manager.doStop)
	runner.AddHandler("stop-workspace", OnDoError(manager.doStop), manager.doStart)
	runner.AddHandler("delete-workspace", OnDoError(manager.doDeleteWorkspace), nil)
	runner.AddHandler("mount-project", OnDoError(manager.doMountProject), manager.undoMountProject)
	runner.AddHandler("delete-workspace-copy", OnDoError(manager.doDeleteWorkspaceCopy), nil)
	runner.AddHandler("make-workspace-copy", OnDoError(manager.doMakeWorkspaceCopy), manager.undoMakeWorkspaceCopy)
	runner.AddHandler("create-state-storage", OnDoError(manager.doCreateStateStorage), manager.doRemoveStateStorage)
	runner.AddHandler("remove-state-storage", OnDoError(manager.doRemoveStateStorage), nil)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
