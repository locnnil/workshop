package statecontext

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

const (
	OpsInProgressKey = "operations-in-progress"
)

type RefreshMode int

const (
	RefreshTransactional RefreshMode = iota
	RefreshWaitOnError
	RefreshContinue
	RefreshAbort
)

func (s RefreshMode) String() string {
	return [...]string{"transactional", "wait-on-error", "continue", "abort"}[s]
}

func ParseRefreshMode(s string) RefreshMode {
	refreshMap := map[string]RefreshMode{
		RefreshTransactional.String(): RefreshTransactional,
		RefreshWaitOnError.String():   RefreshWaitOnError,
		RefreshContinue.String():      RefreshContinue,
		RefreshAbort.String():         RefreshAbort,
	}
	return refreshMap[s]
}

type Operations map[string]Operation

type Operation struct {
	ChangeId    string `json:"changeId"`
	Operation   string `json:"operation"`
	WaitOnError bool   `json:"wait-on-error"`
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

// Attempt to resume the change associated with the refresh operation for the
// given workspace. Depending on the mode the change will either be turned
// into Doing (Continue mode) or Abort (Abort mode)
func ResumeRefresh(st *state.State,
	name string, projectId string, mode RefreshMode) (*state.Change, error) {
	if mode != RefreshAbort && mode != RefreshContinue {
		return nil, fmt.Errorf("cannot resume: only abort or continue can be used to resume the refresh operation")
	}

	op, inProgress := RefreshInProgress(st, name, projectId)
	if !inProgress {
		return nil, fmt.Errorf("cannot %s, no refresh in progress", mode)
	}

	change := st.Change(op.ChangeId)
	if change == nil {
		return nil, fmt.Errorf("cannot %s, no refresh in progress", mode)
	}

	for _, tsk := range change.Tasks() {
		if tsk.Status() == state.WaitStatus {
			if mode == RefreshContinue {
				waited := tsk.WaitedStatus()
				tsk.SetStatus(waited)
				tsk.Logf("Continuing the %q workspace refresh...", name)
			} else if mode == RefreshAbort {
				tsk.Logf("Aborting the %q workspace refresh...", name)
				tsk.SetStatus(state.ErrorStatus)
			}
		}
	}

	if mode == RefreshAbort {
		change.Abort()
	}

	return change, nil
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
