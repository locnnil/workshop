package sdkstate

import (
	"github.com/canonical/workshop/internal/interfaces"
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	backend "github.com/canonical/workshop/internal/workshop"
)

type SdkManager struct {
	backend backend.Backend
	repo    *interfaces.Repository
}

func New(runner *state.TaskRunner, repo *interfaces.Repository, server backend.Backend) *SdkManager {
	manager := &SdkManager{backend: server, repo: repo}

	runner.AddHandler("retrieve-sdk", OnDo(manager.doRetrieveSdk), nil)
	runner.AddHandler("install-sdk", OnDo(manager.doInstallSdk), manager.undoInstallSdk)
	runner.AddHandler("install-host-sdk", OnDo(manager.doInstallHostSdk), manager.undoInstallHostSdk)
	runner.AddHandler("link-sdk", OnDo(manager.doLinkSdk), manager.undoLinkSdk)

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}
