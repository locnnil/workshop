package hookstate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

type HookSetup struct {
	Sdk      workspacebackend.Sdk               `json:"sdk"`
	HookType workspacebackend.WorkspaceHookType `json:"type"`
}

func (h *HookSetup) Type() string {
	return h.HookType.String()
}

type HookManager struct {
	backend workspacebackend.WorkspaceBackend
}

func NewHookManager(runner *state.TaskRunner, server workspacebackend.WorkspaceBackend) *HookManager {
	manager := &HookManager{
		backend: server,
	}

	runner.AddHandler("run-hook", manager.doRunHook, nil)

	return manager
}

func (w *HookManager) Ensure() error {
	return nil
}
