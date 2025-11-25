package sdk

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/progress"
)

type StoreAction int

const (
	Install StoreAction = iota
	Refresh
)

func (s StoreAction) String() string {
	return [...]string{"install", "refresh"}[s]
}

type SdkAction struct {
	ProjectId string
	Workshop  string
	Action    StoreAction
	Name      string
	Base      string
	Channel   string
}

type cachedStoreKey struct{}

// ReplaceStore replaces the store used by the manager.
func ReplaceStore(state *state.State, store Store) {
	state.Lock()
	state.Cache(cachedStoreKey{}, store)
	state.Unlock()
}

func cachedStore(st *state.State) Store {
	sdkStore := st.Cached(cachedStoreKey{})
	if sdkStore == nil {
		return nil
	}
	return sdkStore.(Store)
}

// Store returns the store service provided by the optional device context or
// the one used by the snapstate package if the former has no
// override.
func StoreService(st *state.State) Store {
	if cachedStore := cachedStore(st); cachedStore != nil {
		return cachedStore
	}
	panic("internal error: needing the store before managers have initialized it")
}

type Store interface {
	SdkAction(ctx context.Context, actions []SdkAction) ([]Meta, error)
	DownloadSdk(ctx context.Context, setup Setup, report *progress.Reporter) (*Meta, error)
}

func NewFakeStore() Store {
	return &FakeStore{
		ActionCalls: make([]TestActionCall, 0),
	}
}

type TestActionCall struct {
	Actions []SdkAction
}

type TestDownloadCall struct {
	Setup Setup
}

type FakeStore struct {
	ActionCalls []TestActionCall

	downloadLock  sync.Mutex
	DownloadCalls []TestDownloadCall

	ActionCallback   func(ctx context.Context, actions []SdkAction) ([]Meta, error)
	DownloadCallback func(ctx context.Context, setup Setup, report *progress.Reporter) error
}

func (f *FakeStore) SetActionCallback(fa func(ctx context.Context, actions []SdkAction) ([]Meta, error)) func() {
	old := f.ActionCallback
	f.ActionCallback = fa
	return func() {
		f.ActionCallback = old
	}
}

func (f *FakeStore) SetDownloadCallback(fa func(ctx context.Context, setup Setup, report *progress.Reporter) error) func() {
	old := f.DownloadCallback
	f.DownloadCallback = fa
	return func() {
		f.DownloadCallback = old
	}
}

func (f *FakeStore) SdkAction(ctx context.Context, actions []SdkAction) ([]Meta, error) {
	f.ActionCalls = append(f.ActionCalls, TestActionCall{
		Actions: actions,
	})
	if f.ActionCallback != nil {
		return f.ActionCallback(ctx, actions)
	}
	return nil, nil
}

func (f *FakeStore) DownloadSdk(ctx context.Context, setup Setup, report *progress.Reporter) (*Meta, error) {
	f.downloadLock.Lock()
	defer f.downloadLock.Unlock()
	f.DownloadCalls = append(f.DownloadCalls, TestDownloadCall{
		Setup: setup,
	})
	if f.DownloadCallback != nil {
		if err := f.DownloadCallback(ctx, setup, report); err != nil {
			return nil, err
		}
	}

	sdkYaml := "name: " + setup.Name
	content, err := os.ReadFile(filepath.Join(setup.Filepath(), "meta", "sdk.yaml"))
	if err == nil {
		sdkYaml = string(content)
	}

	return &Meta{Setup: setup, SdkYAML: sdkYaml}, nil
}
