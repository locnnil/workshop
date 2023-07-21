package workspacestate

import (
	"context"

	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"
)

type WorkspaceManager struct {
	backend workspacebackend.WorkspaceBackend
	state   *state.State
}

func NewWorkspaceManager(st *state.State, runner *state.TaskRunner, server workspacebackend.WorkspaceBackend) *WorkspaceManager {
	manager := &WorkspaceManager{
		backend: server,
		state:   st,
	}

	/* Workspace management */
	runner.AddHandler("create-workspace", OnDoError(manager.doCreateWorkspace), manager.undoCreateWorkspace)
	runner.AddHandler("start-workspace", OnDoError(manager.doStart), manager.doStop)
	runner.AddHandler("stop-workspace", OnDoError(manager.doStop), manager.doStart)
	runner.AddHandler("delete-workspace", OnDoError(manager.doDeleteWorkspace), nil)
	runner.AddHandler("mount-project", OnDoError(manager.doMountProject), manager.undoMountProject)
	runner.AddHandler("delete-workspace-copy", OnDoError(manager.doDeleteWorkspaceCopy), nil)
	runner.AddHandler("make-workspace-copy", OnDoError(manager.doMakeWorkspaceCopy), manager.undoMakeWorkspaceCopy)
	runner.AddHandler("create-state-storage", OnDoError(manager.doCreateStateStorage), manager.doRemoveStateStorage)
	runner.AddHandler("remove-state-storage", OnDoError(manager.doRemoveStateStorage), nil)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
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
	op, opInProgress := RefreshInProgress(w.state, ws.Name, ws.ProjectId())
	if opInProgress {
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
