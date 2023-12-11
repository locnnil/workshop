package statecontext

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
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

const (
	OperationLaunch  = "launch"
	OperationRefresh = "refresh"
	OperationStart   = "start"
	OperationStop    = "stop"
	OperationRemove  = "remove"
)

type Operation struct {
	ChangeId    string `json:"changeId"`
	Operation   string `json:"operation"`
	WaitOnError bool   `json:"wait-on-error"`
}

// The family of functions to maintain the state of current operations across
// the workshops. The reason we track the current operations as part of the
// state structure and not as a property of a workshop is that, for example, a
// refresh operation maintains a backup of the previously running workshop.
// Hence, if a workshop was flaged as pending (i.e. refresh op in progress), we
// would have to also make sure that the flag exists in both, its copy of the
// previous instance and the current instance that is created during the refresh
// operation. It involves more complexity on maintaining the workshop state
// record and, likely, makes it more error-prone.

func OperationInProgress(st *state.State, name, projectId string) *Operation {
	var ops Operations
	err := st.Get(OpsInProgressKey, &ops)
	if err != nil {
		return nil
	}

	if op, ok := ops[workshopbackend.InstanceName(name, projectId)]; ok {
		return &op
	}
	return nil
}

func StartOperation(st *state.State, name, projectId string, op Operation) error {
	if cur := OperationInProgress(st, name, projectId); cur != nil {
		return fmt.Errorf("cannot begin %s: %s operation is in progress", op.Operation, cur.Operation)
	}
	var refresh Operations = make(Operations)
	st.Get(OpsInProgressKey, &refresh)
	refresh[workshopbackend.InstanceName(name, projectId)] = op
	st.Set(OpsInProgressKey, refresh)
	return nil
}

// Attempt to resume the change associated with the refresh operation for the
// given workshop. Depending on the mode the change will either be turned
// into Doing (Continue mode) or Abort (Abort mode)
func ResumeRefresh(st *state.State,
	name string, projectId string, mode RefreshMode) (*state.Change, error) {
	if mode != RefreshAbort && mode != RefreshContinue {
		return nil, fmt.Errorf("cannot resume: only abort or continue can be used to resume the refresh operation")
	}

	op := OperationInProgress(st, name, projectId)
	if op == nil {
		return nil, fmt.Errorf("cannot %s, no refresh in progress", mode)
	}

	if op.Operation != OperationRefresh {
		return nil, fmt.Errorf("cannot resume, no refresh in progress (%q is in progress)", op.Operation)
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
				tsk.Logf("Continuing the %q workshop refresh...", name)
			} else if mode == RefreshAbort {
				tsk.Logf("Aborting the %q workshop refresh...", name)
				tsk.SetStatus(state.ErrorStatus)
			}
		}
	}

	if mode == RefreshAbort {
		change.Abort()
	}

	return change, nil
}

// Stop the operation in progress for a given workshop, the state must be
// locked.
func StopOperation(st *state.State, name, projectId, opname string) error {
	var ops Operations
	err := st.Get(OpsInProgressKey, &ops)
	if err != nil {
		return err
	}
	opkey := workshopbackend.InstanceName(name, projectId)
	op, ok := ops[opkey]
	if !ok || opname != op.Operation {
		return fmt.Errorf("cannot finish: no %s in progress", opname)
	}
	delete(ops, opkey)
	st.Set(OpsInProgressKey, ops)
	return nil
}
