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

	runner.AddHandler("create-workspace", manager.doStartBase, nil)
	runner.AddHandler("add-workspace-device", manager.doAddDevice, nil)
	runner.AddHandler("set-workspace-state", manager.doSetState, nil)
	runner.AddHandler("install-sdk", manager.doInstallSDK, manager.undoInstallSDK)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}
