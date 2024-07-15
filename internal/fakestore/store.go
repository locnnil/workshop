package store

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
)

var (
	ErrNoRefreshAvailable = errors.New("SDK has no update available")
)

var (
	SDK_STORE_BUCKET_NAME = "sdk-store"

	storeSdkInfo   = storeSdkInfoImpl
	storeConnect   = storeConnectImpl
	storeSdkReader = storeSdkReaderImpl
)

func New() *GcsStore {
	return &GcsStore{}
}

type GcsStore struct {
}

type storeSdk struct {
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Revision int64  `json:"revision"`
	SdkYAML  string `json:"sdk-yaml"`
}

type SdkActionError struct {
	// maps an SDK name to an error
	errors map[string]error
}

func (e SdkActionError) Error() string {
	errorMsg := []string{"SDK store action failed:"}

	for name, e := range e.errors {
		errorMsg = append(errorMsg, "- "+name+": "+e.Error())
	}
	return strings.Join(errorMsg, "\n")
}

type ClientWrapper struct {
	*storage.Client
	isTesting bool
}

func (c *GcsStore) SdkAction(ctx context.Context, currentSdks map[string]*sdk.Info, actions []sdk.SdkAction) ([]sdk.SdkResult, error) {
	results := []sdk.SdkResult{}
	actError := &SdkActionError{
		errors: make(map[string]error),
	}
	for _, act := range actions {
		switch act.Action {
		case sdk.Install:
			s, err := storeSdkInfo(ctx, act.Name, act.Channel)
			if err != nil {
				actError.errors[act.Name] = err
				continue
			}

			info, err := sdk.ReadSdkInfo([]byte(s.SdkYAML), act.ProjectId, act.Workshop)
			if err != nil {
				actError.errors[act.Name] = err
				continue
			}
			info.Revision = s.Revision
			info.Channel = s.Channel

			results = append(results, sdk.SdkResult{info})
		default:
			return nil, fmt.Errorf("unknown SDK store action")
		}
	}

	if len(actError.errors) > 0 {
		return results, actError
	}

	return results, nil
}

func (c *GcsStore) DownloadSdk(ctx context.Context, setup sdk.Setup) error {
	r, err := storeSdkReader(ctx, setup)
	if err != nil {
		return err
	}
	defer r.Close()

	fl, err := sdk.OpenLock(setup.Name)
	if err != nil {
		return err
	}
	if err = fl.Lock(); err != nil {
		return err
	}
	defer fl.Close()

	target := setup.Filename()
	if !osutil.FileExists(target) {
		file, err := os.Create(target)
		if err != nil {
			return err
		}
		defer func() {
			// Remove the target as due to the error it may be corrupted.
			if err != nil {
				if err1 := os.Remove(target); err1 != nil {
					logger.Noticef("Cannot remove %q on a failed download: %v", target, err1)
				}
				return
			}
			// If the SDK was downloaded successfully, remove its previous rev if any.
			matches, err1 := filepath.Glob(filepath.Join(filepath.Dir(target), setup.Name+"_*.sdk"))
			if err1 != nil {
				logger.Noticef("Cannot cleanup previous downloads for %q: %v", setup.Name, err1)
			}
			for _, m := range matches {
				if m != target {
					if err1 = os.Remove(m); err1 != nil {
						logger.Noticef("Cannot cleanup previous download (%s): %v", m, err1)
					}
				}
			}
		}()
		defer file.Close()

		if _, err = io.Copy(file, r); err != nil {
			return err
		}
	} else {
		logger.Debugf("%s exists, nothing to download...", target)
	}

	return nil
}

func storeConnectImpl(ctx context.Context) (*ClientWrapper, error) {
	opt := option.WithoutAuthentication()
	testing := false
	if url := os.Getenv("SDK_STORE_URL"); url != "" { // Set STORAGE_EMULATOR_HOST environment variable for GSC.
		err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:8080")
		if err != nil {
			return nil, err
		}
		opt = option.WithEndpoint(url)
		testing = true
	}
	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		return nil, err
	}
	return &ClientWrapper{client, testing}, nil
}

func storeSdkInfoImpl(ctx context.Context, name, channel string) (storeSdk, error) {
	var sSdk storeSdk
	client, err := storeConnect(ctx)
	if err != nil {
		return sSdk, err
	}
	defer client.Close()
	bkt := client.Bucket(SDK_STORE_BUCKET_NAME)

	var sa = strings.Split(channel, "/")
	if len(sa) != 2 {
		return sSdk, fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	}
	track, risk := sa[0], sa[1]
	obj := bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", name, track, risk, name))
	atr, err := obj.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return sSdk, errors.New("SDK not found")
		}
		return sSdk, err
	}
	sSdk.Name = name
	sSdk.Channel = channel
	// A simple modulo to keep revision numbers in a readable form for testing
	sSdk.Revision = atr.Generation % 1000
	// The test server for the SDK store cannot store metadata.
	if !client.isTesting {
		if _, ok := atr.Metadata["sdk-yaml"]; !ok {
			return sSdk, fmt.Errorf("SDK %q does not have metadata", name)
		}
		sSdk.SdkYAML = atr.Metadata["sdk-yaml"]
	} else {
		// Emulate meta data for the test SDKs. We need the name/base pair for
		// the e2e scenarios to work.
		sSdk.SdkYAML, err = readTestMetadata(ctx, client, name, track, risk)
		if err != nil {
			return sSdk, err
		}
	}
	return sSdk, nil
}

// Only relevant for the end to end tests
func readTestMetadata(ctx context.Context, client *ClientWrapper, name, track, risk string) (string, error) {
	var b bytes.Buffer
	meta := bufio.NewWriter(&b)

	bkt := client.Bucket(SDK_STORE_BUCKET_NAME)
	obj := bkt.Object(fmt.Sprintf("%s/%s/%s/sdk.yaml", name, track, risk))
	r, err := obj.NewReader(ctx)
	if err != nil {
		return "", err
	}
	defer r.Close()

	if _, err = io.Copy(meta, r); err != nil {
		return "", err
	}
	return b.String(), nil
}

func storeSdkReaderImpl(ctx context.Context, setup sdk.Setup) (io.ReadCloser, error) {
	client, err := storeConnect(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var sa = strings.Split(setup.Channel, "/")
	if len(sa) != 2 {
		return nil, fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", setup.Name, setup.Channel)
	}
	track, risk := sa[0], sa[1]
	bkt := client.Bucket(SDK_STORE_BUCKET_NAME)
	obj := bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", setup.Name, track, risk, setup.Name))
	r, err := obj.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, errors.New("SDK not found")
		}
		return nil, err
	}
	return r, nil
}
