package statecontext

import (
	"context"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/tomb.v2"
)

type HandlerDecorator func(handler state.HandlerFunc) state.HandlerFunc

func WaitOnErrorDecorator(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		_, p, ws, err := UserProjectWorkspace(task)
		if err != nil {
			return err
		}

		err = handler(task, tomb)
		if err != nil {
			st := task.State()
			st.Lock()
			defer st.Unlock()

			op, inProgress := RefreshInProgress(st, ws, p.ProjectId)
			if inProgress && op.WaitOnError {
				return &state.Wait{
					WaitedStatus: state.DoingStatus,
					Reason:       fmt.Sprintf("wait on error %v", err),
				}
			} else {
				StopRefresh(st, ws, p.ProjectId)
			}
		}
		return err
	}
}

func AddHandler(runner *state.TaskRunner, kind string, do, undo state.HandlerFunc, decor HandlerDecorator) {
	if decor != nil {
		var doHandler, undoHandler state.HandlerFunc
		if do != nil {
			doHandler = decor(do)
		}
		if undo != nil {
			undoHandler = decor(undo)
		}
		runner.AddHandler(kind, doHandler, undoHandler)
	} else {
		runner.AddHandler(kind, do, undo)
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
