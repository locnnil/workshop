package hookstate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/workspacebackend"
)

type HookManager struct {
	backend srv.WorkspaceBackend
}

func NewHookManager(runner *state.TaskRunner, server srv.WorkspaceBackend) *HookManager {
	manager := &HookManager{
		backend: server,
	}

	runner.AddHandler("run-hook", manager.doRunHook, nil)

	return manager
}

func (w *HookManager) Ensure() error {
	return nil
}
