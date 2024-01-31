package hookstate

import (
	"fmt"
	"sync"
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
	Workshop    string            `json:"workshop"`
	Sdk         string            `json:"sdk"`
	HookType    WorkshopHookType  `json:"type"`
	Environment map[string]string `json:"environment"`
	Timeout     time.Duration     `json:"timeout"`
	IgnoreError bool              `json:"bool"`
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
	state   *state.State
	backend workshopbackend.WorkshopBackend

	contextsMutex sync.RWMutex
	contexts      map[string]*Context
}

func New(s *state.State, runner *state.TaskRunner, server workshopbackend.WorkshopBackend) *HookManager {
	manager := &HookManager{
		state:   s,
		backend: server,
	}

	runner.AddHandler("run-hook", OnDo(manager.doRunHook), nil)

	return manager
}

func (w *HookManager) Ensure() error {
	return nil
}

func (m *HookManager) ephemeralContext(cookieID string) (context *Context, err error) {
	var contexts map[string]string
	m.state.Lock()
	defer m.state.Unlock()
	err = m.state.Get("workshop-cookies", &contexts)
	if err != nil {
		return nil, fmt.Errorf("cannot get workshop cookies: %v", err)
	}
	if workshop, ok := contexts[cookieID]; ok {
		// create new ephemeral context
		context, err = NewContext(nil, m.state, &HookSetup{Workshop: workshop}, nil, cookieID)
		return context, err
	}
	return nil, fmt.Errorf("invalid workshop cookie requested")
}

// Context obtains the context for the given cookie ID.
func (m *HookManager) Context(cookieID string) (*Context, error) {
	m.contextsMutex.RLock()
	defer m.contextsMutex.RUnlock()

	var err error
	context, ok := m.contexts[cookieID]
	if !ok {
		context, err = m.ephemeralContext(cookieID)
		if err != nil {
			return nil, err
		}
	}

	return context, nil
}
