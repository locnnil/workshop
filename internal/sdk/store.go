package sdk

import (
	"context"
	"crypto/sha3"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"regexp"
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
	Download(ctx context.Context, w io.Writer, sdk sdkstore.SdkArchive, options ...sdkstore.DownloadOption) error
	Find(ctx context.Context, query string, options ...sdkstore.FindOption) ([]transport.FindResponse, error)
	Info(ctx context.Context, name string, options ...sdkstore.InfoOption) (transport.InfoResponse, error)
	Resolve(ctx context.Context, request transport.ResolveRequest) (transport.ResolveResponse, error)
}

func NewFakeStore() *FakeStore {
	return &FakeStore{}
}

type FakeStore struct {
	lock sync.Mutex

	DownloadCalls    []sdkstore.SdkArchive
	DownloadCallback func(ctx context.Context, w io.Writer, sdk sdkstore.SdkArchive, options ...sdkstore.DownloadOption) error

	FindCalls    []string
	FindCallback func(ctx context.Context, query string, options ...sdkstore.FindOption) ([]transport.FindResponse, error)

	InfoCalls    []string
	InfoCallback func(ctx context.Context, name string, options ...sdkstore.InfoOption) (transport.InfoResponse, error)

	ResolveCalls    []transport.ResolveRequest
	ResolveCallback func(ctx context.Context, req transport.ResolveRequest) (transport.ResolveResponse, error)
}

func (f *FakeStore) SetDownloadCallback(download func(ctx context.Context, w io.Writer, sdk sdkstore.SdkArchive, options ...sdkstore.DownloadOption) error) func() {
	f.lock.Lock()
	defer f.lock.Unlock()

	old := f.DownloadCallback
	f.DownloadCallback = download
	return func() {
		f.DownloadCallback = old
	}
}

func (f *FakeStore) Download(ctx context.Context, w io.Writer, sdk sdkstore.SdkArchive, options ...sdkstore.DownloadOption) error {
	f.lock.Lock()
	f.DownloadCalls = append(f.DownloadCalls, sdk)
	download := f.DownloadCallback
	f.lock.Unlock()

	if download == nil {
		return fmt.Errorf("%q SDK (%v) not found", sdk.Name, sdk.Revision)
	}
	return download(ctx, w, sdk, options...)
}

func (f *FakeStore) SetFindCallback(find func(ctx context.Context, query string, options ...sdkstore.FindOption) ([]transport.FindResponse, error)) func() {
	f.lock.Lock()
	defer f.lock.Unlock()

	old := f.FindCallback
	f.FindCallback = find
	return func() {
		f.FindCallback = old
	}
}

func (f *FakeStore) Find(ctx context.Context, query string, options ...sdkstore.FindOption) ([]transport.FindResponse, error) {
	f.lock.Lock()
	f.FindCalls = append(f.FindCalls, query)
	find := f.FindCallback
	f.lock.Unlock()

	if find == nil {
		return nil, nil
	}
	return find(ctx, query, options...)
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

func (f *FakeStore) SetResolveCallback(resolve func(ctx context.Context, req transport.ResolveRequest) (transport.ResolveResponse, error)) func() {
	f.lock.Lock()
	defer f.lock.Unlock()

	old := f.ResolveCallback
	f.ResolveCallback = resolve
	return func() {
		f.ResolveCallback = old
	}
}

func (f *FakeStore) Resolve(ctx context.Context, req transport.ResolveRequest) (transport.ResolveResponse, error) {
	f.lock.Lock()
	f.ResolveCalls = append(f.ResolveCalls, req)
	resolve := f.ResolveCallback
	f.lock.Unlock()

	if resolve == nil {
		return transport.ResolveResponse{}, errors.New("resolve not implemented")
	}
	return resolve(ctx, req)
}

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// FakePackageID generates a deterministic string which looks like a package
// ID from the Store, but isn't. In some cases it might not be long enough.
func FakePackageID(name string) string {
	digest := sha3.Sum384([]byte(name))
	packageID := base64.RawStdEncoding.EncodeToString(digest[:])
	packageID = nonAlphanumeric.ReplaceAllString(packageID, "")
	if len(packageID) > 32 {
		packageID = packageID[:32]
	}
	return packageID
}

func FakeResolve(sdks map[string]Meta) func(ctx context.Context, req transport.ResolveRequest) (transport.ResolveResponse, error) {
	return func(ctx context.Context, req transport.ResolveRequest) (transport.ResolveResponse, error) {
		return fakeResolve(req, sdks)
	}
}

func fakeResolve(req transport.ResolveRequest, sdks map[string]Meta) (transport.ResolveResponse, error) {
	responses := make([]transport.ResolvePackageResponse, 0, len(req.Packages))

	for _, pkg := range req.Packages {
		resp := transport.ResolvePackageResponse{
			InstanceKey: pkg.InstanceKey,
			Namespace:   "sdk",
			Name:        pkg.Name,
		}

		sk, ok := sdks[pkg.Name]
		if ok {
			resp.Status = "ok"
			resp.ID = sk.PackageID
			resp.Result = transport.ResolvePackageResult{
				Channel: transport.ResolvePackageChannel{
					Name:             pkg.Channel,
					EffectiveChannel: pkg.Channel,
					Platform:         pkg.Platform,
				},
				Revision: transport.ResolveRevision{
					Platforms: []transport.Platform{pkg.Platform},
					Download: transport.Download{
						Sha3_384: sk.Sha3_384,
					},
					Revision: sk.Revision.N,
				},
			}
		} else {
			resp.Status = "error"
			resp.Error = &transport.APIError{
				Code:    "package-not-found",
				Message: "Package not found",
			}
		}

		responses = append(responses, resp)
	}

	rand.Shuffle(len(responses), func(i, j int) {
		responses[i], responses[j] = responses[j], responses[i]
	})
	return transport.ResolveResponse{PackageResults: responses}, nil
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
