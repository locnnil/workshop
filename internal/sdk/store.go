package sdk

import (
	"context"

	"github.com/canonical/workshop/internal/overlord/state"
)

type StoreAction int

const (
	Install StoreAction = iota
	Refresh
)

func (s StoreAction) String() string {
	return [...]string{"install", "refresh"}[s]
}

type SdkResult struct {
	*Info
}

type SdkAction struct {
	ProjectId string
	Workshop  string
	Action    StoreAction
	Name      string
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
	SdkAction(ctx context.Context, currentSdks map[string]*Info, actions []SdkAction) ([]SdkResult, error)
	DownloadSdk(ctx context.Context, setup Setup) error
}

func NewFakeStore() Store {
	return &FakeStore{
		ActionCalls: make([]TestActionCall, 0),
	}
}

type TestActionCall struct {
	CurrentSdks map[string]*Info
	Actions     []SdkAction
}

type TestDownloadCall struct {
	Setup Setup
}

type FakeStore struct {
	ActionCalls      []TestActionCall
	DownloadCalls    []TestDownloadCall
	ActionCallback   func(ctx context.Context, currentSdks map[string]*Info, actions []SdkAction) ([]SdkResult, error)
	DownloadCallback func(ctx context.Context, setup Setup) error
}

func (f *FakeStore) SdkAction(ctx context.Context, currentSdks map[string]*Info, actions []SdkAction) ([]SdkResult, error) {
	f.ActionCalls = append(f.ActionCalls, TestActionCall{
		CurrentSdks: currentSdks,
		Actions:     actions,
	})
	if f.ActionCallback != nil {
		return f.ActionCallback(ctx, currentSdks, actions)
	}
	return nil, nil
}

func (f *FakeStore) DownloadSdk(ctx context.Context, setup Setup) error {
	f.DownloadCalls = append(f.DownloadCalls, TestDownloadCall{
		Setup: setup,
	})
	if f.DownloadCallback != nil {
		return f.DownloadCallback(ctx, setup)
	}
	return nil
}
