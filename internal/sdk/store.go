package sdk

import (
	"context"
	"sync"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdkstore"
	"github.com/canonical/workshop/internal/sdkstore/transport"
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

// ReplaceStore replaces the SDK store used by the manager.
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

// StoreService returns the active store service.
func StoreService(st *state.State) Store {
	if store := cachedStore(st); store != nil {
		return store
	}
	panic("internal error: needing the store before managers have initialized it")
}

type Store interface {
	Info(ctx context.Context, name string, options ...sdkstore.InfoOption) (transport.InfoResponse, error)
}

func NewFakeStore() *FakeStore {
	return &FakeStore{}
}

type FakeStore struct {
	lock sync.Mutex

	InfoCalls    []string
	InfoCallback func(ctx context.Context, name string, options ...sdkstore.InfoOption) (transport.InfoResponse, error)
}

func (f *FakeStore) SetInfoCallback(info func(ctx context.Context, name string, options ...sdkstore.InfoOption) (transport.InfoResponse, error)) func() {
	f.lock.Lock()
	defer f.lock.Unlock()

	old := f.InfoCallback
	f.InfoCallback = info
	return func() {
		f.InfoCallback = old
	}
}

func (f *FakeStore) Info(ctx context.Context, name string, options ...sdkstore.InfoOption) (transport.InfoResponse, error) {
	f.lock.Lock()
	f.InfoCalls = append(f.InfoCalls, name)
	info := f.InfoCallback
	f.lock.Unlock()

	if info == nil {
		return transport.InfoResponse{}, &sdkstore.SdkNotFoundError{Name: name}
	}
	return info(ctx, name, options...)
}

type cachedGcsStoreKey struct{}

// ReplaceGcsStore replaces the GCS store used by the manager.
func ReplaceGcsStore(state *state.State, store GcsStore) {
	state.Lock()
	state.Cache(cachedGcsStoreKey{}, store)
	state.Unlock()
}

func cachedGcsStore(st *state.State) GcsStore {
	sdkStore := st.Cached(cachedGcsStoreKey{})
	if sdkStore == nil {
		return nil
	}
	return sdkStore.(GcsStore)
}

// GcsStoreService returns the store service provided by the optional device
// context or the one used by the snapstate package if the former has no
// override.
func GcsStoreService(st *state.State) GcsStore {
	if store := cachedGcsStore(st); store != nil {
		return store
	}
	panic("internal error: needing the store before managers have initialized it")
}

type GcsStore interface {
	SdkAction(ctx context.Context, actions []SdkAction) ([]Meta, error)
	DownloadSdk(ctx context.Context, setup Setup, report *progress.Reporter) (*Meta, error)
}

func NewFakeGcsStore() *FakeGcsStore {
	return &FakeGcsStore{
		ActionCalls: make([]TestActionCall, 0),
	}
}

type TestActionCall struct {
	Actions []SdkAction
}

type TestDownloadCall struct {
	Setup Setup
}

type FakeGcsStore struct {
	ActionCalls []TestActionCall

	downloadLock  sync.Mutex
	DownloadCalls []TestDownloadCall

	ActionCallback   func(ctx context.Context, actions []SdkAction) ([]Meta, error)
	DownloadCallback func(ctx context.Context, setup Setup, report *progress.Reporter) (*Meta, error)
}

func (f *FakeGcsStore) SetActionCallback(fa func(ctx context.Context, actions []SdkAction) ([]Meta, error)) func() {
	old := f.ActionCallback
	f.ActionCallback = fa
	return func() {
		f.ActionCallback = old
	}
}

func (f *FakeGcsStore) SetDownloadCallback(fa func(ctx context.Context, setup Setup, report *progress.Reporter) (*Meta, error)) func() {
	old := f.DownloadCallback
	f.DownloadCallback = fa
	return func() {
		f.DownloadCallback = old
	}
}

func (f *FakeGcsStore) SdkAction(ctx context.Context, actions []SdkAction) ([]Meta, error) {
	f.ActionCalls = append(f.ActionCalls, TestActionCall{
		Actions: actions,
	})
	if f.ActionCallback != nil {
		return f.ActionCallback(ctx, actions)
	}
	return nil, nil
}

func (f *FakeGcsStore) DownloadSdk(ctx context.Context, setup Setup, report *progress.Reporter) (*Meta, error) {
	f.downloadLock.Lock()
	defer f.downloadLock.Unlock()
	f.DownloadCalls = append(f.DownloadCalls, TestDownloadCall{
		Setup: setup,
	})
	if f.DownloadCallback != nil {
		return f.DownloadCallback(ctx, setup, report)
	}

	return &Meta{Setup: setup, SdkYAML: "name: " + setup.Name}, nil
}
