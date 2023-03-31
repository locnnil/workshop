package sdkstate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	backend "github.com/canonical/workspace/internal/workspacebackend"
)

type SdkManager struct {
	backend backend.WorkspaceBackend
}

type SdkSequenceRecord struct {
	Channel  string `json:"channel"`
	Revision int64  `json:"revision"`
}

func NewSdkManager(runner *state.TaskRunner, server backend.WorkspaceBackend) *SdkManager {
	manager := &SdkManager{backend: server}

	runner.AddHandler("retrieve-sdk", manager.doRetrieveSdk, nil)
	runner.AddHandler("install-sdk", manager.doInstallSDK, manager.undoInstallSdk)
	runner.AddHandler("link-sdk", manager.doLinkSdk, manager.undoLinkSdk)

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
