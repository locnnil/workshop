package statecontext

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/tomb.v2"
)

// The Do handler decoractor that helps to decide whether:
// 1. The task needs to be put on Wait (wait-on-error for refresh).
// 2. The error needs to be reported but safely ingored (ContextCancelled can
// happen if a user cancells or something gets interrupted during the execution
// due to abortion, e.g. a running hook is called off because their change was
// aborted.
// 3. The error needs to be reported as is which will cause the abortion.
func OnDo(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		_, p, ws, err := UserProjectWorkspace(task)
		if err != nil {
			return err
		}

		err = handler(task, tomb)
		st := task.State()
		st.Lock()
		defer st.Unlock()

		switch {
		case err == nil:
			if task.Has("stop-operation") {
				op := OperationInProgress(st, ws, p.ProjectId)
				if e := StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
					return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
				}
			}
			return nil
		case errors.Is(err, context.Canceled):
			task.Logf("The task execution was cancelled")
			// the context cancellation here means the change was aborted and
			// the undo logic chain started. we don't report the context
			// cancellation as error here as it is an expected interruption
			op := OperationInProgress(st, ws, p.ProjectId)
			if op != nil {
				if e := StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
					return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
				}
			}
			return nil
		case err != nil:
			op := OperationInProgress(st, ws, p.ProjectId)
			if op != nil {
				if op.Operation == OperationRefresh && op.WaitOnError {
					task.Logf("Setting the task to wait until the refresh is either aborted or continued...")
					task.Errorf("%v", err)
					return &state.Wait{
						WaitedStatus: state.DoingStatus,
						Reason:       fmt.Sprintf("wait on error: %v", err),
					}
				}

				if e := StopOperation(st, ws, p.ProjectId, op.Operation); e != nil {
					return fmt.Errorf("internal error: cannot stop %s for %q: %v, error: %v", op.Operation, ws, e, err)
				}
			}

			return err
		}

		return nil
	}
}

func UserProjectWorkspace(task *state.Task) (string, *workspacebackend.Project, string, error) {
	st := task.State()
	var prj workspacebackend.Project
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
	err = task.Get("workspace", &name)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get workspace, task %q: %v", id, err)
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

func BackendContext(tomb *tomb.Tomb, user string, prj *workspacebackend.Project) (context.Context, context.CancelFunc) {
	ctx := tomb.Context(context.Background())
	ctxProject := context.WithValue(ctx, workspacebackend.ContextProjectId, prj.ProjectId)
	ctxUser := context.WithValue(ctxProject, workspacebackend.ContextUser, user)
	ctxCancel, cancel := context.WithCancel(ctxUser)
	return ctxCancel, cancel
}
