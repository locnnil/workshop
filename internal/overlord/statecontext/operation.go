package statecontext

import (
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

const (
	OpsInProgressKey = "op-in-progress"
)

type Operations map[string]Operation

type Operation struct {
	ChangeId    string   `json:"changeId"`
	Operation   string   `json:"operation"`
	WaitOnError bool     `json:"wait-on-error"`
	Errors      []string `json:"error"`
}

// The family of functions to maintain the state of current operations across
// the workspaces. The reason we track the current operations as part of the
// state structure and not as a property of a workspace is that, for example, a
// refresh operation maintains a backup of the previously running workspace.
// Hence, if a workspace was flaged as pending (i.e. refresh op in progress), we
// would have to also make sure that the flag exists in both, its copy of the
// previous instance and the current instance that is created during the refresh
// operation. It involves more complexity on maintaining the workspace state
// record and, likely, makes it more error-prone.

// Returns an associated refresh operation for the workspace. An empty
// string will be returned if no refresh in progress. The state must be locked
func RefreshInProgress(st *state.State, name, projectId string) (*Operation, bool) {
	var ops Operations
	err := st.Get(OpsInProgressKey, &ops)
	if err != nil {
		return nil, false
	}
	op := ops[workspacebackend.InstanceName(name, projectId)]
	if op.Operation != "refresh" {
		return nil, false
	}
	return &op, true
}

// Sets a given workspace to the refresh mode, the state must be locked. The
// method associates the workspace with a change id that can be used to continue
// or abort the refresh operation later on.
func StartRefresh(st *state.State, name, projectId, change string, wait bool) error {
	var refresh Operations = make(Operations)
	var setup = Operation{ChangeId: change, Operation: "refresh", WaitOnError: wait}

	st.Get(OpsInProgressKey, &refresh)

	refresh[workspacebackend.InstanceName(name, projectId)] = setup
	st.Set(OpsInProgressKey, refresh)
	return nil
}

// Unset the refresh mode for a given workspace, the state must be locked. The
// method removes an association between a workspace and a change indicating
// that the refresh is over and continue or abort will not be possible from this
// point.
func StopRefresh(st *state.State, name, projectId string) error {
	var ops Operations
	err := st.Get(OpsInProgressKey, &ops)
	if err != nil {
		return err
	}
	delete(ops, workspacebackend.InstanceName(name, projectId))
	st.Set(OpsInProgressKey, ops)
	return nil
}

func WorkspaceState(st *state.State, ws *workspacebackend.Workspace) workspacebackend.WorkspaceState {
	op, opInProgress := RefreshInProgress(st, ws.Name, ws.ProjectId())
	if opInProgress {
		if ws.IsRunning() {
			change := st.Change(op.ChangeId)
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
