package sdkstate

import (
	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	backend "github.com/canonical/workshop/internal/workshopbackend"
)

type SdkManager struct {
	backend backend.WorkshopBackend
}

type SdkSequenceRecord struct {
	Channel  string `json:"channel"`
	Revision int64  `json:"revision"`
}

func New(runner *state.TaskRunner, server backend.WorkshopBackend) *SdkManager {
	manager := &SdkManager{backend: server}

	runner.AddHandler("retrieve-sdk", OnDo(manager.doRetrieveSdk), nil)
	runner.AddHandler("install-sdk", OnDo(manager.doInstallSDK), OnUndo(manager.undoInstallSdk))
	runner.AddHandler("link-sdk", OnDo(manager.doLinkSdk), OnUndo(manager.undoLinkSdk))

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
