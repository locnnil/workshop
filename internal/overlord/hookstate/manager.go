package hookstate

import (
	"time"

	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"
)

// Handler is the interface a client must satify to handle hooks.
type Handler interface {
	// Before is called right before the hook is to be run.
	Before() error

	// Done is called right after the hook has finished successfully.
	Done() error

	// Error is called if the hook encounters an error while running.
	// The returned bool flag indicates if the original hook error should be
	// ignored by hook manager.
	Error(hookErr error) (ignoreHookErr bool, err error)
}

type HookSetup struct {
	Sdk         workshopbackend.SdkRecord `json:"sdk"`
	HookType    WorkshopHookType          `json:"type"`
	Environment map[string]string         `json:"environment"`
	Timeout     time.Duration             `json:"timeout"`
	IgnoreError bool                      `json:"bool"`
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
