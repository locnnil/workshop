package conflict

import (
	"errors"
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

type ChangeSetup struct {
	Mode string `json:"mode"`
}

type Mode int

const (
	ChangeTransactional Mode = iota
	ChangeWaitOnError
	ChangeContinue
	ChangeAbort
)

func (s Mode) String() string {
	return [...]string{"transactional", "wait-on-error", "continue", "abort"}[s]
}

func (s Mode) Resume() bool {
	return s == ChangeContinue || s == ChangeAbort
}

func ParseMode(s string) (Mode, error) {
	changeMap := map[string]Mode{
		ChangeTransactional.String(): ChangeTransactional,
		ChangeWaitOnError.String():   ChangeWaitOnError,
		ChangeContinue.String():      ChangeContinue,
		ChangeAbort.String():         ChangeAbort,
	}
	if val, ok := changeMap[s]; ok {
		return val, nil
	}
	return -1, errors.New(`change mode must be any of: "transactional", "wait-on-error", "continue", "abort"`)
}

// ChangeConflictError represents an error because of snap conflicts between changes.
type ChangeConflictError struct {
	ProjectId  string
	Workshop   string
	ChangeKind string
	// a Message is optional, otherwise one is composed from the other information
	Message string
	// ChangeID can optionally be set to the ID of the change with which the operation conflicts
	ChangeID string
}

func (e *ChangeConflictError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ChangeKind != "" {
		return fmt.Sprintf("workshop %q has %q change in progress", e.Workshop, e.ChangeKind)
	}
	return fmt.Sprintf("workshop %q has changes in progress", e.Workshop)
}

func checkWorkshop(task *state.Task, projectId, workshop string) (bool, error) {
	chg := task.Change()

	if task.Kind() == "disconnect" {
		// disconnect can affect more then one workshop
		var plugRef sdk.PlugRef
		var slotRef sdk.SlotRef
		if err := task.Get("plug", &plugRef); err != nil {
			return false, err
		}
		if err := task.Get("slot", &slotRef); err != nil {
			return false, err
		}

		if projectId == plugRef.ProjectId && workshop == plugRef.Workshop {
			return true, nil
		}

		if projectId == slotRef.ProjectId && workshop == slotRef.Workshop {
			return true, nil
		}
		return false, nil
	}

	if !chg.Has("project-id") || !task.Has("workshop") {
		return false, nil
	}

	var taskWorkshop, chgProject string
	if err := task.Get("workshop", &taskWorkshop); err != nil {
		return false, fmt.Errorf("internal error: cannot obtain workshop name from task: %s", task.Summary())
	}

	if err := chg.Get("project-id", &chgProject); err != nil {
		return false, fmt.Errorf("internal error: cannot obtain project from task: %s", task.Summary())
	}

	if projectId == chgProject && workshop == taskWorkshop {
		return true, nil
	}
	return false, nil
}

// Iterates over the list of running tasks and returns a ChangeConflictError if
// there is another change running for the provided projectID / workshop pair.
func CheckChangeConflict(st *state.State, projectId, workshop string, ignoreChange string) error {
	for _, task := range st.Tasks() {
		chg := task.Change()
		if chg.IsReady() || chg.ID() == ignoreChange {
			continue
		}

		ok, err := checkWorkshop(task, projectId, workshop)
		if err != nil {
			return err
		}
		if ok {
			return &ChangeConflictError{
				ProjectId:  projectId,
				Workshop:   workshop,
				ChangeKind: chg.Kind(),
				ChangeID:   chg.ID(),
			}
		}

	}
	return nil
}

// Attempt to resume the change associated with the Resume/Launch operation
// for the given workshop. Depending on the mode the change will either be
// turned into Doing (Continue mode) or Abort (Abort mode).
func ResumeAfterWait(st *state.State,
	workshop string, projectId string, mode Mode, action string) (*state.Change, error) {
	if mode != ChangeAbort && mode != ChangeContinue {
		return nil, fmt.Errorf("cannot resume: only abort or continue can be used to resume the operation")
	}

	var chg *state.Change
	for _, task := range st.Tasks() {
		if task.Change().IsReady() {
			continue
		}

		if ok, err := checkWorkshop(task, projectId, workshop); err != nil {
			return nil, err
		} else if ok {
			chg = task.Change()
			break
		}
	}
	if chg == nil {
		return nil, fmt.Errorf("cannot %s: no wait in progress", mode)
	}

	if chg.Kind() != action {
		return nil, fmt.Errorf("cannot %s: %s requested but %s is in progress", mode, action, chg.Kind())
	}

	if chg.Kind() != "refresh" && chg.Kind() != "launch" {
		return nil, fmt.Errorf("cannot %s: no wait in progress (%q is in progress)", chg.Kind(), mode)
	}

	if chg.Status() != state.WaitStatus {
		return nil, fmt.Errorf("cannot %s: no wait in progress", mode)
	}

	for _, tsk := range chg.Tasks() {
		if tsk.Status() == state.WaitStatus {
			if mode == ChangeContinue {
				waited := tsk.WaitedStatus()
				tsk.SetStatus(waited)
				tsk.Logf("Continuing for workshop %q...", workshop)
			} else if mode == ChangeAbort {
				tsk.SetStatus(state.DoStatus)
				tsk.Logf("Aborting for workshop %q...", workshop)
			}
		}
	}

	if mode == ChangeAbort {
		chg.Abort()
	}

	return chg, nil
}
