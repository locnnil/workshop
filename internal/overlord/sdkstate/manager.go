package sdkstate

import (
	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	backend "github.com/canonical/workshop/internal/workshopbackend"
)

type SdkManager struct {
	backend backend.WorkshopBackend
}

func New(runner *state.TaskRunner, server backend.WorkshopBackend) *SdkManager {
	manager := &SdkManager{backend: server}

	runner.AddHandler("retrieve-sdk", OnDo(manager.doRetrieveSdk), manager.undoRetrieveSdk)
	runner.AddHandler("install-sdk", OnDo(manager.doInstallSDK), manager.undoInstallSdk)
	runner.AddHandler("link-sdk", OnDo(manager.doLinkSdk), manager.undoLinkSdk)

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
