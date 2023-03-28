package workspace

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

	runner.AddHandler("create-workspace", manager.doCreateWorkspace, manager.undoCreateWorkspace)
	runner.AddHandler("mount-project", manager.doMountProject, nil)
	runner.AddHandler("start-workspace", manager.doStart, manager.doStop)
	runner.AddHandler("install-sdk", manager.doInstallSDK, manager.undoInstallSdk)
	runner.AddHandler("link-sdk", manager.doLinkSdk, manager.undoLinkSdk)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
