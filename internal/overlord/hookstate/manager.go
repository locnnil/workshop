package hookstate

import (
	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"
)

type HookSetup struct {
	Sdk         workshopbackend.SdkRecord `json:"sdk"`
	HookType    WorkspaceHookType         `json:"type"`
	Environment map[string]string         `json:"environment"`
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
	backend workshopbackend.WorkspaceBackend
}

func New(runner *state.TaskRunner, server workshopbackend.WorkspaceBackend) *HookManager {
	manager := &HookManager{
		backend: server,
	}

	runner.AddHandler("run-hook", OnDo(manager.doRunHook), nil)

	return manager
}

func (w *HookManager) Ensure() error {
	return nil
}
