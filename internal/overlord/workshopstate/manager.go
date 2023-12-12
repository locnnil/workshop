package workshopstate

import (
	"context"

	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"
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

	/* Workshop management */
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

func (w *WorkshopManager) Ensure() error {
	return nil
}

// Checks all of the provided list of workshops are in the required status as
// per the matchStatus predicate. It returns the first workshop that does NOT
// meet the predicate's condition.
func (w *WorkshopManager) CheckStatus(ctx context.Context, names []string, pId string,
	matchStatus func(status workshopbackend.WorkshopStatus) bool) (string, workshopbackend.WorkshopStatus, error) {
	for _, name := range names {
		wrkspc, err := w.Workshop(ctx, name, pId)
		if err != nil {
			return "", workshopbackend.WorkshopOff, err
		}

		status := w.workshopStatus(wrkspc)
		if !matchStatus(status) {
			return name, status, nil
		}
	}
	return "", workshopbackend.WorkshopOff, nil
}

// Loads a workshop, the state must be locked as it is used to find out the
// workshop state
func (w *WorkshopManager) Workshop(ctx context.Context, name, pId string) (*workshopbackend.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workshopbackend.ContextProjectId, pId)

	wrkspc, err := w.backend.Workshop(pCtx, name)
	if err != nil {
		return nil, err
	}

	wrkspc.SetStatus(w.workshopStatus(wrkspc))
	return wrkspc, nil
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

	for _, wrkspc := range workshops {
		wrkspc.SetStatus(w.workshopStatus(wrkspc))
	}

	return files, workshops, nil
}

// Infers the state of a workshop based on the container's state and any of the
// operations in progress for the workshop. The state must be locked before the
// call.
func (w *WorkshopManager) workshopStatus(ws *workshopbackend.Workshop) workshopbackend.WorkshopStatus {
	op := OperationInProgress(w.state, ws.Name, ws.ProjectId())
	if op != nil {
		if ws.IsRunning() {
			change := w.state.Change(op.ChangeId)
			if change == nil {
				return workshopbackend.WorkshopError
			}
			if change.Status() == state.WaitStatus {
				ws.AddError(workshopbackend.WaitOnError)
				return workshopbackend.WorkshopPending
			}
			if len(ws.Errors()) == 0 {
				return workshopbackend.WorkshopPending
			}
			return workshopbackend.WorkshopError
		} else {
			if len(ws.Errors()) > 0 {
				return workshopbackend.WorkshopError
			}
			return workshopbackend.WorkshopPending
		}
	} else {
		if ws.IsRunning() && len(ws.Errors()) == 0 {
			return workshopbackend.WorkshopReady
		}
		if len(ws.Errors()) > 0 {
			return workshopbackend.WorkshopError
		}
		return workshopbackend.WorkshopStopped
	}
}
