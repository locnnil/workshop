package workspacestate

import (
	"context"

	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workspacebackend"
)

type WorkspaceManager struct {
	backend workspacebackend.WorkspaceBackend
	state   *state.State
}

func New(st *state.State, runner *state.TaskRunner, server workspacebackend.WorkspaceBackend) *WorkspaceManager {
	manager := &WorkspaceManager{
		backend: server,
		state:   st,
	}

	/* Workshop management */
	runner.AddHandler("create-workshop", OnDo(manager.doCreateWorkspace), OnUndo(manager.undoCreateWorkspace))
	runner.AddHandler("start-workshop", OnDo(manager.doStart), OnUndo(manager.doStop))
	runner.AddHandler("stop-workshop", OnDo(manager.doStop), OnUndo(manager.doStart))
	runner.AddHandler("remove-workshop", OnDo(manager.doRemoveWorkspace), nil)
	runner.AddHandler("mount-project", OnDo(manager.doMountProject), OnUndo(manager.undoMountProject))
	runner.AddHandler("remove-workshop-stash", OnDo(manager.doRemoveWorkspaceStash), nil)
	runner.AddHandler("stash-workshop", OnDo(manager.doStashWorkspace), OnUndo(manager.undoStashWorkspace))
	runner.AddHandler("create-state-storage", OnDo(manager.doCreateStateStorage), OnUndo(manager.doRemoveStateStorage))
	runner.AddHandler("remove-state-storage", OnDo(manager.doRemoveStateStorage), nil)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}

// Checks all of the provided list of workspaces are in the required status as
// per the matchStatus predicate. It returns the first workshop that does NOT
// meet the predicate's condition.
func (w *WorkspaceManager) CheckStatus(ctx context.Context, names []string, pId string,
	matchStatus func(status workspacebackend.WorkspaceState) bool) (string, workspacebackend.WorkspaceState, error) {
	for _, name := range names {
		wrkspc, err := w.Workshop(ctx, name, pId)
		if err != nil {
			return "", workspacebackend.WorkspaceOff, err
		}

		status := w.workspaceState(wrkspc)
		if !matchStatus(status) {
			return name, status, nil
		}
	}
	return "", workspacebackend.WorkspaceOff, nil
}

// Loads a workshop, the state must be locked as it is used to find out the
// workshop state
func (w *WorkspaceManager) Workshop(ctx context.Context, name, pId string) (*workspacebackend.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workspacebackend.ContextProjectId, pId)

	wrkspc, err := w.backend.GetWorkspace(pCtx, name)
	if err != nil {
		return nil, err
	}

	wrkspc.SetState(w.workspaceState(wrkspc))
	return wrkspc, nil
}

// Loads all workspaces for a project, the state must be locked as it is used to find out the
// workshop state
func (w *WorkspaceManager) Workspaces(ctx context.Context, pId string) ([]*workspacebackend.WorkspaceFile, []*workspacebackend.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workspacebackend.ContextProjectId, pId)

	files, workspaces, err := w.backend.GetProjectWorkspaces(pCtx)
	if err != nil {
		return nil, nil, err
	}

	for _, wrkspc := range workspaces {
		wrkspc.SetState(w.workspaceState(wrkspc))
	}

	return files, workspaces, nil
}

// Infers the state of a workshop based on the container's state and any of the
// operations in progress for the workshop. The state must be locked before the
// call.
func (w *WorkspaceManager) workspaceState(ws *workspacebackend.Workshop) workspacebackend.WorkspaceState {
	op := OperationInProgress(w.state, ws.Name, ws.ProjectId())
	if op != nil {
		if ws.IsRunning() {
			change := w.state.Change(op.ChangeId)
			if change == nil {
				return workspacebackend.WorkspaceError
			}
			if change.Status() == state.WaitStatus {
				ws.AddError(workspacebackend.WaitOnError)
				return workspacebackend.WorkspacePending
			}
			if len(ws.Errors()) == 0 {
				return workspacebackend.WorkspacePending
			}
			return workspacebackend.WorkspaceError
		} else {
			if len(ws.Errors()) > 0 {
				return workspacebackend.WorkspaceError
			}
			return workspacebackend.WorkspacePending
		}
	} else {
		if ws.IsRunning() && len(ws.Errors()) == 0 {
			return workspacebackend.WorkspaceReady
		}
		if len(ws.Errors()) > 0 {
			return workspacebackend.WorkspaceError
		}
		return workspacebackend.WorkspaceStopped
	}
}
