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

func NewSdkManager(runner *state.TaskRunner, server backend.WorkspaceBackend) *SdkManager {
	manager := &SdkManager{backend: server}

	AddHandler(runner, "retrieve-sdk", manager.doRetrieveSdk, nil, WaitOnErrorDecorator)
	AddHandler(runner, "install-sdk", manager.doInstallSDK, manager.undoInstallSdk, WaitOnErrorDecorator)
	AddHandler(runner, "link-sdk", manager.doLinkSdk, manager.undoLinkSdk, WaitOnErrorDecorator)

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
