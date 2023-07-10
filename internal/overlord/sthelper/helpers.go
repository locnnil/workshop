package sharedstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/tomb.v2"
)

type HandlerDecorator func(handler state.HandlerFunc) state.HandlerFunc

func WaitOnErrorDecorator(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		err := handler(task, tomb)
		if err != nil {
			st := task.State()
			st.Lock()
			defer st.Unlock()
			var waitOnErr = false
			if errors.Is(task.Change().Get("wait-on-error", &waitOnErr), state.ErrNoState) {
				return err
			}

			if waitOnErr {
				return &state.Wait{
					WaitedStatus: state.DoingStatus,
					Reason:       "the change is in the wait-on-error mode and will wait for the user input",
				}
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

func SdkSetup(task *state.Task) (*sdk.SdkInfo, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var retrieveId string
	var blob sdk.SdkInfo

	err := task.Get("sdk-retrieve-task", &retrieveId)

	if err != nil {
		return nil, err
	}

	retrieve := task.State().Task(retrieveId)
	if retrieve == nil {
		return nil, fmt.Errorf("internal error: no corresponding retrieve-sdk task found")
	}

	if err = retrieve.Get("sdk-setup", &blob); err != nil {
		return nil, err
	}
	return &blob, nil
}

func UserProjectWorkspace(task *state.Task) (string, *workspacebackend.Project, string, error) {
	st := task.State()
	var prj workspacebackend.Project
	var name string
	var user string

	st.Lock()
	id := task.ID()
	err := task.Get("project-key", &prj)
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
