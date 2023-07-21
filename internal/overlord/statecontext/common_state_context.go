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
func OnDoError(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		_, p, ws, err := UserProjectWorkspace(task)
		if err != nil {
			return err
		}

		err = handler(task, tomb)
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				st := task.State()
				st.Lock()
				defer st.Unlock()

				task.Logf("cannot proceed: task execution cancelled")
				return nil

			case err != nil:
				st := task.State()
				st.Lock()
				defer st.Unlock()

				op, inProgress := RefreshInProgress(st, ws, p.ProjectId)
				if inProgress && op.WaitOnError {
					task.Logf("%q workspace refresh failed, resolve errors before resuming", ws)
					task.Errorf(err.Error())
					return &state.Wait{
						WaitedStatus: state.DoingStatus,
						Reason:       fmt.Sprintf("wait on error: %v", err),
					}
				} else if inProgress {
					if e := StopRefresh(st, ws, p.ProjectId); e != nil {
						return fmt.Errorf("internal error: cannot stop refresh for %q: %v, refresh error: %v", ws, e, err)
					}
				}
				return err
			}
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
