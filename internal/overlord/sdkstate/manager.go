package sdkstate

import (
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	backend "github.com/canonical/workshop/internal/workshop"
)

type SdkManager struct {
	backend backend.Backend
}

func New(runner *state.TaskRunner, server backend.Backend) *SdkManager {
	manager := &SdkManager{backend: server}

	runner.AddHandler("retrieve-sdk", OnDo(manager.doRetrieveSdk), nil)
	runner.AddHandler("install-sdk", OnDo(manager.doInstallSDK), manager.undoInstallSdk)
	runner.AddHandler("link-sdk", OnDo(manager.doLinkSdk), manager.undoLinkSdk)

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
