package hookstate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"
)

type HookSetup struct {
	Sdk         workspacebackend.Sdk `json:"sdk"`
	HookType    WorkspaceHookType    `json:"type"`
	Environment map[string]string    `json:"environment"`
}

type WorkspaceHookType int

const (
	SetupBase WorkspaceHookType = iota
	SaveState
	RestoreState
)

func (s WorkspaceHookType) String() string {
	return [...]string{"setup-base", "save-state", "restore-state"}[s]
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

	runner.AddHandler("run-hook", OnDo(manager.doRunHook), nil)

	return manager
}

func (w *HookManager) Ensure() error {
	return nil
}
