package sdkstate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
	backend "github.com/canonical/workspace/internal/workspacebackend"
)

type SdkManager struct {
	backend backend.WorkspaceBackend
}

type SdkSequenceRecord struct {
	Channel  string `json:"channel"`
	Revision int64  `json:"revision"`
}

func New(runner *state.TaskRunner, server backend.WorkspaceBackend) *SdkManager {
	manager := &SdkManager{backend: server}

	runner.AddHandler("retrieve-sdk", OnDo(manager.doRetrieveSdk), nil)
	runner.AddHandler("install-sdk", OnDo(manager.doInstallSDK), OnUndo(manager.undoInstallSdk))
	runner.AddHandler("link-sdk", OnDo(manager.doLinkSdk), OnUndo(manager.undoLinkSdk))

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
