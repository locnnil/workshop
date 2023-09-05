package workspacestate

import (
	"context"
	"sync"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"
)

type WorkspaceManager struct {
	backend workspacebackend.WorkspaceBackend
	state   *state.State

	execChannelsLock sync.Mutex
	execChannels     map[string]chan bool
}

func NewWorkspaceManager(st *state.State, runner *state.TaskRunner, server workspacebackend.WorkspaceBackend) *WorkspaceManager {
	manager := &WorkspaceManager{
		backend:      server,
		state:        st,
		execChannels: make(map[string]chan bool),
	}

	/* Workspace management */
	runner.AddHandler("create-workspace", OnDo(manager.doCreateWorkspace), OnUndo(manager.undoCreateWorkspace))
	runner.AddHandler("start-workspace", OnDo(manager.doStart), OnUndo(manager.doStop))
	runner.AddHandler("stop-workspace", OnDo(manager.doStop), OnUndo(manager.doStart))
	runner.AddHandler("remove-workspace", OnDo(manager.doRemoveWorkspace), nil)
	runner.AddHandler("mount-project", OnDo(manager.doMountProject), OnUndo(manager.undoMountProject))
	runner.AddHandler("remove-workspace-stash", OnDo(manager.doRemoveWorkspaceStash), nil)
	runner.AddHandler("stash-workspace", OnDo(manager.doStashWorkspace), OnUndo(manager.undoStashWorkspace))
	runner.AddHandler("create-state-storage", OnDo(manager.doCreateStateStorage), OnUndo(manager.doRemoveStateStorage))
	runner.AddHandler("remove-state-storage", OnDo(manager.doRemoveStateStorage), nil)
	runner.AddHandler("exec", OnDo(manager.doExecCommand), nil)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}

// Checks all of the provided list of workspaces are in the required status as
// per the matchStatus predicate. It returns the first workspace that does NOT
// meet the predicate's condition.
func (w *WorkspaceManager) CheckStatus(ctx context.Context, names []string, pId string,
	matchStatus func(status workspacebackend.WorkspaceState) bool) (string, workspacebackend.WorkspaceState, error) {
	for _, name := range names {
		wrkspc, err := w.Workspace(ctx, name, pId)
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

// Loads a workspace, the state must be locked as it is used to find out the
// workspace state
func (w *WorkspaceManager) Workspace(ctx context.Context, name, pId string) (*workspacebackend.Workspace, error) {
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
// workspace state
func (w *WorkspaceManager) Workspaces(ctx context.Context, pId string) ([]*workspacebackend.WorkspaceFile, []*workspacebackend.Workspace, error) {
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

// Infers the state of a workspace based on the container's state and any of the
// operations in progress for the workspace. The state must be locked before the
// call.
func (w *WorkspaceManager) workspaceState(ws *workspacebackend.Workspace) workspacebackend.WorkspaceState {
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

func (w *WorkspaceManager) WaitExecReady(ctx context.Context, execTaskId string, timeout time.Duration) error {
	w.execChannelsLock.Lock()
	execCh := w.execChannels[execTaskId]
	w.execChannelsLock.Unlock()
	if execCh == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-execCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
