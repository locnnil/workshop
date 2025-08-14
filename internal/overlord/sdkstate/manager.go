package sdkstate

import (
	"time"

	"github.com/canonical/workshop/internal/interfaces"
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

type SdkManager struct {
	backend workshop.Backend
	repo    *interfaces.Repository
}

var (
	sdkVolumeCooldownTime = 1 * time.Hour
)

func New(s *state.State, runner *state.TaskRunner, repo *interfaces.Repository) *SdkManager {
	manager := &SdkManager{repo: repo}

	s.Lock()
	manager.backend = workshop.WorkshopBackend(s)
	s.Unlock()

	runner.AddHandler("retrieve-sdk", OnDo(manager.doRetrieveSdk), nil)
	runner.AddHandler("install-sdk", OnDo(manager.doInstallSdk), OnUndo(manager.doUninstallSdk))
	runner.AddHandler("register-sdk", OnDo(manager.doRegisterSdk), OnUndo(manager.doUnregisterSdk))
	runner.AddHandler("unregister-sdk", OnDo(manager.doUnregisterSdk), OnUndo(manager.doRegisterSdk))

	runner.AddCleanup("unregister-sdk", manager.doDeleteUnusedSdkVolumes)

	return manager
}

func (w *SdkManager) Ensure() error {
	return nil
}

func FakeSdkVolumeCooldownTime(t time.Duration) (restore func()) {
	old := sdkVolumeCooldownTime
	sdkVolumeCooldownTime = t
	return func() {
		sdkVolumeCooldownTime = old
	}
}
