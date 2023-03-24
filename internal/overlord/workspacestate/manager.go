package workspace

import (
	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/server"
)

type WorkspaceManager struct {
	server srv.WorkspaceServer
}

func NewWorkspaceManager(runner *state.TaskRunner, server srv.WorkspaceServer) *WorkspaceManager {
	manager := &WorkspaceManager{
		server: server,
	}

	runner.AddHandler("create-workspace", manager.doCreateWorkspace, manager.undoCreateWorkspace)
	runner.AddHandler("mount-project", manager.doMountProject, nil)
	runner.AddHandler("start-workspace", manager.doStart, manager.doStop)
	runner.AddHandler("install-sdk", manager.doInstallSDK, manager.undoInstallSdk)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
