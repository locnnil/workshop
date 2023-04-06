package sharedstate

import (
	"context"
	"fmt"

	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/canonical/workspace/internal/overlord/state"
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

func ProjectAndWorkspace(task *state.Task) (*projectstate.ProjectKey, string, error) {
	st := task.State()
	var project projectstate.ProjectKey
	var name string

	st.Lock()
	err := task.Change().Get("project-key", &project)
	st.Unlock()

	if err != nil {
		return nil, "", fmt.Errorf("cannot get project for task %q: %v", task.ID(), err)
	}

	st.Lock()
	err = task.Change().Get("workspace", &name)
	st.Unlock()

	if err != nil {
		return nil, "", fmt.Errorf("cannot get workspace for task %q: %v", task.ID(), err)
	}

	return &project, name, nil
}

func BackendContext(tomb *tomb.Tomb, project *projectstate.ProjectKey) (context.Context, context.CancelFunc) {
	ctx := tomb.Context(context.Background())
	ctxProject := context.WithValue(ctx, workspacebackend.ContextProjectId, project.ProjectId)
	ctxCancel, cancel := context.WithCancel(ctxProject)
	return ctxCancel, cancel
}
