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
	UnknownStatus HealthStatus = iota
	ReadyStatus
	PendingStatus
	ErrorStatus
	StoppedStatus
	OffStatus
)

func (s HealthStatus) String() string {
	statuses := [...]string{"Unknown", "Ready", "Pending", "Error", "Stopped", "Off"}
	if s < 0 || int(s) >= len(statuses) {
		return fmt.Sprintf("invalid (%d)", s)
	}
	return statuses[s]
}

var knownSetHealthStatuses = []string{"unknown", "okay", "waiting", "error"}

func SetHealthStatusLookup(str string) (HealthStatus, error) {
	switch str {
	case "okay":
		return ReadyStatus, nil
	case "waiting":
		return PendingStatus, nil
	case "error":
		return ErrorStatus, nil
	case "unknown":
		return UnknownStatus, nil
	}

	return -1, fmt.Errorf("invalid status %q, must be one of %s", str, strutil.Quoted(knownSetHealthStatuses))
}

type HealthState struct {
	Timestamp time.Time    `json:"timestamp"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Code      string       `json:"code,omitempty"`
	Sdk       string       `json:"sdk,omitempty"`
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
			Sdk:     h.context.Sdk(),
		})
	}
	var counter int
	err = h.context.Get("retry-counter", &counter)
	if errors.Is(err, state.ErrNoState) {
		h.context.Set("retry-counter", 0)
	}
	return nil
}

var retryTimeout = 1 * time.Second
var retriesAllowed = 10

func (h *healthHandler) Done() error {
	var health HealthState
	var retryCounter int

	h.context.Lock()
	err := h.context.Get("health", &health)
	h.context.Unlock()

	if err != nil {
		// note it can't actually be state.ErrNoState because Before sets it
		return err
	}

	h.context.Lock()
	err = h.context.Get("retry-counter", &retryCounter)
	h.context.Unlock()

	if err != nil {
		// note it can't actually be state.ErrNoState because Before sets it
		return err
	}

	if retryCounter >= retriesAllowed && (health.Status == PendingStatus || health.Status == UnknownStatus) {
		return fmt.Errorf("SDK %q status is not healthy after multiple checks", h.context.Sdk())
	}

	if health.Status == PendingStatus || health.Status == UnknownStatus {
		h.context.Lock()
		h.context.Set("retry-counter", retryCounter+1)
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
