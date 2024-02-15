package workshopstate

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/operation"
	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/canonical/x-go/strutil"
)

type WorkshopManager struct {
	backend workshopbackend.WorkshopBackend
	state   *state.State
}

func New(st *state.State, runner *state.TaskRunner, server workshopbackend.WorkshopBackend) *WorkshopManager {
	manager := &WorkshopManager{
		backend: server,
		state:   st,
	}

	runner.AddHandler("create-workshop", OnDo(manager.doCreateWorkshop), OnUndo(manager.undoCreateWorkshop))
	runner.AddHandler("start-workshop", OnDo(manager.doStart), OnUndo(manager.doStop))
	runner.AddHandler("stop-workshop", OnDo(manager.doStop), OnUndo(manager.doStart))
	runner.AddHandler("remove-workshop", OnDo(manager.doRemoveWorkshop), nil)
	runner.AddHandler("mount-project", OnDo(manager.doMountProject), OnUndo(manager.undoMountProject))
	runner.AddHandler("remove-workshop-stash", OnDo(manager.doRemoveWorkshopStash), nil)
	runner.AddHandler("stash-workshop", OnDo(manager.doStashWorkshop), OnUndo(manager.undoStashWorkshop))
	runner.AddHandler("create-state-storage", OnDo(manager.doCreateStateStorage), OnUndo(manager.doRemoveStateStorage))
	runner.AddHandler("remove-state-storage", OnDo(manager.doRemoveStateStorage), nil)

	return manager
}

func (m *WorkshopManager) StartUp() error {
	return nil
}

func (w *WorkshopManager) Ensure() error {
	return nil
}

// Checks all of the provided list of workshops are in the required health status.
func (w *WorkshopManager) CheckStatus(ctx context.Context, names []string, pId string, allowedStatuses []healthstate.Status) error {
	for _, name := range names {
		workshop, err := w.Workshop(ctx, name, pId)
		if err != nil {
			return err
		}

		health := w.WorkshopHealth(workshop)
		allowed := []string{}
		for _, s := range allowedStatuses {
			allowed = append(allowed, s.String())
		}
		if slices.Index(allowedStatuses, health.Status) == -1 {
			return fmt.Errorf("%q status is %q, must be one of: %s", name, health.Status.String(), strutil.Quoted(allowed))
		}
	}
	return nil
}

// Loads a workshop, the state must be locked as it is used to find out the
// workshop state
func (w *WorkshopManager) Workshop(ctx context.Context, name, pId string) (*workshopbackend.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workshopbackend.ContextProjectId, pId)

	workshop, err := w.backend.Workshop(pCtx, name)
	if err != nil {
		return nil, err
	}

	return workshop, nil
}

// Loads all workshops for a project, the state must be locked as it is used to find out the
// workshop state
func (w *WorkshopManager) Workshops(ctx context.Context, pId string) ([]*workshopbackend.WorkshopFile, []*workshopbackend.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workshopbackend.ContextProjectId, pId)

	files, workshops, err := w.backend.ProjectWorkshops(pCtx)
	if err != nil {
		return nil, nil, err
	}

	return files, workshops, nil
}

// Examine the tasks of the change to fetch possible check-health hook results
// for the workshop's SDKs.
func sdksHealthCheckSummary(chg *state.Change) map[string]healthstate.HealthCheck {
	var SdkChecks = map[string]healthstate.HealthCheck{}
	for _, task := range chg.Tasks() {
		if task.Kind() == "run-hook" {
			var healthCheck healthstate.HealthCheck
			if err := task.Get("health", &healthCheck); err == nil {
				SdkChecks[healthCheck.Sdk] = healthCheck
			}
		}
	}
	return SdkChecks
}

// Infers the state of a workshop based on the container's state and any of the
// operations in progress for the workshop. The state must be locked before the
// call.
func (w *WorkshopManager) WorkshopHealth(ws *workshopbackend.Workshop) healthstate.HealthState {
	var healthState = healthstate.HealthState{
		Timestamp: time.Now(),
	}

	// check the project directory exists
	if !ws.Project().Exists() {
		healthState.Status = healthstate.ErrorStatus
		healthState.Code = "missing-project"
		return healthState
	}

	// check if the workshop file exists
	if _, err := ws.File(); err != nil {
		healthState.Status = healthstate.ErrorStatus
		healthState.Code = "missing-file"
		return healthState
	}

	op := operation.OperationInProgress(w.state, ws.Name, ws.Project().ProjectId)
	if op != nil {
		change := w.state.Change(op.ChangeId)
		// a change could have gone as a result of pruning TODO: when prune
		// changes, abort all the outstanding operations connected with it
		if change == nil {
			healthState.Status = healthstate.ErrorStatus
			return healthState
		}
		if change.Status() == state.WaitStatus {
			healthState.Code = "wait-on-error"
		}

		healthState.SdkHealth = sdksHealthCheckSummary(change)
		healthState.Status = healthstate.PendingStatus
	} else {
		if ws.IsRunning() {
			healthState.Status = healthstate.ReadyStatus
		} else {
			healthState.Status = healthstate.StoppedStatus
		}
	}
	return healthState
}
