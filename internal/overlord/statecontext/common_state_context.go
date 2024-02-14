package statecontext

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workshop/internal/overlord/operation"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"

	"gopkg.in/tomb.v2"
)

// The Do handler decoractor that helps to decide whether:
// 1. The task needs to be put on Wait (wait-on-error for refresh).
// 2. The error needs to be reported but safely ingored (ContextCancelled can
// happen if a user cancells or something gets interrupted during the execution
// due to abortion, e.g. a running hook is called off because their change was
// aborted.
// 3. The error needs to be reported as is which will abort the change (or the
// affected lanes).
func OnDo(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		_, p, ws, err := UserProjectWorkshop(task)
		if err != nil {
			return err
		}

		err = handler(task, tomb)
		st := task.State()
		st.Lock()
		defer st.Unlock()

		switch {
		case err == nil:
			// see if the task finishes the chain of tasks representing an
			// operation launch, refresh, etc.
			if task.Has("stop-operation") {
				op := operation.OperationInProgress(st, ws, p.ProjectId)
				if op != nil {
					if e := operation.StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
						return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
					}
				}
			}
			return nil
		case errors.Is(err, context.Canceled):
			task.Logf("The task execution was cancelled")
			// the context cancellation here means the change was aborted and
			// the undo logic chain started. we don't report the context
			// cancellation as error here as it is an expected interruption
			op := operation.OperationInProgress(st, ws, p.ProjectId)
			if op != nil {
				if e := operation.StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
					return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
				}
			}
			return nil
		case err != nil:
			op := operation.OperationInProgress(st, ws, p.ProjectId)
			if op != nil {
				if op.Operation == operation.OperationRefresh && op.WaitOnError {
					task.Logf("Setting the task to wait until the refresh is either aborted or continued...")
					task.Errorf("%v", err)
					return &state.Wait{
						WaitedStatus: state.DoingStatus,
						Reason:       fmt.Sprintf("wait on error: %v", err),
					}
				}

				if e := operation.StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
					return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
				}
			}

			return err
		}

		return nil
	}
}

func OnUndo(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		_, p, ws, err := UserProjectWorkshop(task)
		if err != nil {
			return err
		}

		err = handler(task, tomb)
		st := task.State()
		st.Lock()
		defer st.Unlock()

		// if the task was marked as the starter of the operation then
		// remove the operation from being in progress as this is the last
		// task that has just completed its undoing logic, i.e. the
		// workshop is ready for the new commands again
		if task.Has("start-operation") {
			op := operation.OperationInProgress(st, ws, p.ProjectId)
			if op != nil {
				if e := operation.StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
					return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
				}
			}
		}
		return err
	}
}

func UserProjectWorkshop(task *state.Task) (string, *workshopbackend.Project, string, error) {
	st := task.State()
	var prj workshopbackend.Project
	var name string
	var user string

	st.Lock()
	id := task.ID()
	err := task.Get("project", &prj)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get project, task %q: %v", id, err)
	}

	st.Lock()
	err = task.Get("workshop", &name)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get workshop, task %q: %v", id, err)
	}

	st.Lock()
	changeId := task.Change().ID()
	err = task.Change().Get("user", &user)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get user name, change %q: %v", changeId, err)
	}

	return user, &prj, name, nil
}

func BackendContext(tomb *tomb.Tomb, user string, prj *workshopbackend.Project) (context.Context, context.CancelFunc) {
	ctx := tomb.Context(context.Background())
	ctxProject := context.WithValue(ctx, workshopbackend.ContextProjectId, prj.ProjectId)
	ctxUser := context.WithValue(ctxProject, workshopbackend.ContextUser, user)
	ctxCancel, cancel := context.WithCancel(ctxUser)
	return ctxCancel, cancel
}
