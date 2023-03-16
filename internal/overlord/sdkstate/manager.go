package sdkstate

import (
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/overlord/state"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"gopkg.in/tomb.v2"
)

type SdkManager struct {
}

func NewManager(runner *state.TaskRunner) *SdkManager {
	manager := &SdkManager{}

	runner.AddHandler("retrieve-sdk", manager.doRetrieveSdk, nil)
	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}

func (m *SdkManager) doRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	var sdk workspace.Sdk

	st.Lock()
	err := task.Get("sdk", &sdk)
	st.Unlock()

	if err != nil {
		return err
	}

	client, err := store.NewStoreClient()
	if err != nil {
		return nil
	}

	blob, err := client.RetrieveSdk(sdk.Name, sdk.Channel)
	if err != nil {
		return err
	}

	st.Lock()
	task.Set("sdk-blob", blob)
	st.Unlock()

	return nil
}
