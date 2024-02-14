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

type Status int

const (
	UnknownStatus Status = iota
	ReadyStatus
	PendingStatus
	ErrorStatus
	StoppedStatus
	OffStatus
)

func (s Status) String() string {
	statuses := [...]string{"Unknown", "Ready", "Pending", "Error", "Stopped", "Off"}
	if s < 0 || int(s) >= len(statuses) {
		return fmt.Sprintf("invalid (%d)", s)
	}
	return statuses[s]
}

type HealthCheckResult int

const (
	CheckUnknown HealthCheckResult = iota
	CheckWaiting
	CheckOkay
	CheckError
)

var knownSetHealthStatuses = []string{"unknown", "waiting", "okay", "error"}

func SetHealthLookup(str string) (HealthCheckResult, error) {
	switch str {
	case "okay":
		return CheckOkay, nil
	case "waiting":
		return CheckWaiting, nil
	case "error":
		return CheckError, nil
	case "unknown":
		return CheckUnknown, nil
	}

	return -1, fmt.Errorf("invalid status %q, must be one of %s", str, strutil.Quoted(knownSetHealthStatuses))
}

func (s HealthCheckResult) String() string {
	if s < 0 || int(s) >= len(knownSetHealthStatuses) {
		return fmt.Sprintf("invalid (%d)", s)
	}
	return knownSetHealthStatuses[s]
}

type HealthCheck struct {
	Sdk         string            `json:"sdk"`
	Timestamp   time.Time         `json:"timestamp"`
	CheckResult HealthCheckResult `json:"check-result"`
	Message     string            `json:"message,omitempty"`
	Code        string            `json:"code,omitempty"`
}

type HealthState struct {
	Timestamp time.Time              `json:"timestamp"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Code      string                 `json:"code,omitempty"`
	SdkHealth map[string]HealthCheck `json:"sdk-health,omitempty"`
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
	var health HealthCheck
	err := h.context.Get("health", &health)
	// the handler is being called for the first time
	// (the health is set already if it is a retry)
	if errors.Is(err, state.ErrNoState) {
		h.context.Set("health", HealthCheck{
			Sdk:         h.context.Sdk(),
			CheckResult: CheckUnknown,
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
	var health HealthCheck
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

	if retryCounter >= retriesAllowed && (health.CheckResult == CheckWaiting || health.CheckResult == CheckUnknown) {
		return fmt.Errorf("SDK %q is not healthy after multiple checks", h.context.Sdk())
	}

	if health.CheckResult == CheckWaiting || health.CheckResult == CheckUnknown {
		h.context.Lock()
		h.context.Set("retry-counter", retryCounter+1)
		h.context.Unlock()
		return &state.Retry{After: retryTimeout, Reason: health.Message}
	}

	if health.CheckResult == CheckError {
		return errors.New(health.Message)
	}

	return nil
}

func (h *healthHandler) Error(err error) (bool, error) {
	return false, nil
}
