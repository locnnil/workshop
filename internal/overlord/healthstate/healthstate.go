package healthstate

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/x-go/strutil"
)

type HealthStatus int

const (
	UnknownStatus = HealthStatus(iota)
	OkayStatus
	WaitingStatus
	ErrorStatus
)

var knownStatuses = []string{"unknown", "okay", "waiting", "error"}

func StatusLookup(str string) (HealthStatus, error) {
	for i, k := range knownStatuses {
		if k == str {
			return HealthStatus(i), nil
		}
	}
	return -1, fmt.Errorf("invalid status %q, must be one of %s", str, strutil.Quoted(knownStatuses))
}

func (s HealthStatus) String() string {
	if s < 0 || s >= HealthStatus(len(knownStatuses)) {
		return fmt.Sprintf("invalid (%d)", s)
	}
	return knownStatuses[s]
}

type HealthState struct {
	Timestamp time.Time    `json:"timestamp"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Code      string       `json:"code,omitempty"`
	Retries   int          `json:"retries,omitempty"`
}

func Init(hookManager *hookstate.HookManager) {
	hookManager.Register(regexp.MustCompile("^check-health$"), newHealthHandler)
}

func newHealthHandler(ctx *hookstate.Context) hookstate.Handler {
	return &healthHandler{context: ctx}
}

type healthHandler struct {
	context *hookstate.Context
}

// Before is called just before the hook runs -- nothing to do beyond setting a marker
func (h *healthHandler) Before() error {
	h.context.Lock()
	defer h.context.Unlock()
	var health HealthState
	err := h.context.Get("health", &health)
	// the handler is being called for the first time
	// (the health is set already if it is a retry)
	if errors.Is(err, state.ErrNoState) {
		h.context.Set("health", HealthState{
			Status:  UnknownStatus,
			Message: "The health status is unknown",
		})
	}
	return nil
}

var retryTimeout = 1 * time.Second
var retriesAllowed = 10

func (h *healthHandler) Done() error {
	var health HealthState

	h.context.Lock()
	err := h.context.Get("health", &health)
	h.context.Unlock()

	if err != nil {
		// note it can't actually be state.ErrNoState because Before sets it
		return err
	}

	if health.Retries >= retriesAllowed && (health.Status == WaitingStatus || health.Status == UnknownStatus) {
		return fmt.Errorf("SDK %q status is not healthy after multiple checks", h.context.Sdk())
	}

	if health.Status == WaitingStatus || health.Status == UnknownStatus {
		health.Retries = health.Retries + 1
		h.context.Lock()
		h.context.Set("health", health)
		h.context.Unlock()
		return &state.Retry{After: retryTimeout, Reason: health.Message}
	}

	if health.Status == ErrorStatus {
		return errors.New(health.Message)
	}

	return nil
}

func (h *healthHandler) Error(err error) (bool, error) {
	return false, nil
}
