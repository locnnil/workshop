package sdk

import (
	"context"
	"crypto/md5"
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"hash"
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

type SdkResult struct {
	Setup
	Sha3_384 string
	MD5      string
	SdkYAML  string
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
	SdkAction(ctx context.Context, actions []SdkAction) ([]SdkResult, error)
	DownloadSdk(ctx context.Context, setup Setup, report *progress.Reporter) (*SdkResult, error)
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

	ActionCallback   func(ctx context.Context, actions []SdkAction) ([]SdkResult, error)
	DownloadCallback func(ctx context.Context, setup Setup, report *progress.Reporter) error
}

func (f *FakeStore) SetActionCallback(fa func(ctx context.Context, actions []SdkAction) ([]SdkResult, error)) func() {
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

func (f *FakeStore) SdkAction(ctx context.Context, actions []SdkAction) ([]SdkResult, error) {
	f.ActionCalls = append(f.ActionCalls, TestActionCall{
		Actions: actions,
	})
	if f.ActionCallback != nil {
		return f.ActionCallback(ctx, actions)
	}
	return nil, nil
}

func (f *FakeStore) DownloadSdk(ctx context.Context, setup Setup, report *progress.Reporter) (*SdkResult, error) {
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

	source, err := setup.Source.MarshalText()
	if err != nil {
		return nil, err
	}

	var hash hash.Hash
	if IsSystem(setup.Name) {
		hash = sha3.New384()
	} else {
		hash = md5.New()
	}
	fmt.Fprintf(hash, "%s:%s:%s:%s", setup.Name, source, setup.Channel, setup.Revision)
	digest := hex.EncodeToString(hash.Sum(nil))

	result := &SdkResult{Setup: setup, SdkYAML: "name: " + setup.Name}
	if IsSystem(setup.Name) {
		result.Sha3_384 = digest
	} else {
		result.MD5 = digest
	}
	return result, nil
}
