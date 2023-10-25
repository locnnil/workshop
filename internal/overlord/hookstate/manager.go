package hookstate

import (
	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"
)

type HookSetup struct {
	Sdk         workshopbackend.SdkRecord `json:"sdk"`
	HookType    WorkshopHookType          `json:"type"`
	Environment map[string]string         `json:"environment"`
}

type WorkshopHookType int

const (
	SetupBase WorkshopHookType = iota
	SaveState
	RestoreState
)

func (s WorkshopHookType) String() string {
	return [...]string{"setup-base", "save-state", "restore-state"}[s]
}

func (h *HookSetup) Type() string {
	return h.HookType.String()
}

type HookManager struct {
	backend workshopbackend.WorkshopBackend
}

func New(runner *state.TaskRunner, server workshopbackend.WorkshopBackend) *HookManager {
	manager := &HookManager{
		backend: server,
	}

	runner.AddHandler("run-hook", OnDo(manager.doRunHook), nil)

	return manager
}

func (w *HookManager) Ensure() error {
	return nil
}
