package sharedstate

import (
	"context"
	"fmt"

	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/tomb.v2"
)

func SdkSetup(task *state.Task) (*store.SdkBlob, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var retrieveId string
	var blob store.SdkBlob

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

func UserProjectWorkspace(task *state.Task) (string, *project.Project, string, error) {
	st := task.State()
	var prj project.Project
	var name string
	var user string

	st.Lock()
	id := task.Change().ID()
	err := task.Change().Get("project-key", &prj)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get project, change %q: %v", id, err)
	}

	st.Lock()
	err = task.Change().Get("workspace", &name)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get workspace, change %q: %v", id, err)
	}

	st.Lock()
	err = task.Change().Get("user", &user)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get user name, change %q: %v", id, err)
	}

	return user, &prj, name, nil
}

func BackendContext(tomb *tomb.Tomb, user string, prj *project.Project) (context.Context, context.CancelFunc) {
	ctx := tomb.Context(context.Background())
	ctxProject := context.WithValue(ctx, workspacebackend.ContextProjectId, prj.ProjectId)
	ctxUser := context.WithValue(ctxProject, workspacebackend.ContextUser, user)
	ctxCancel, cancel := context.WithCancel(ctxUser)
	return ctxCancel, cancel
}
