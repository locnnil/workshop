package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
)

var (
	ErrNoRefreshAvailable = errors.New("SDK has no update available")
)

var storeSdkInfo = storeSdkInfoImpl

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
	errorMsg := []string{}

	for _, e := range e.errors {
		errorMsg = append(errorMsg, e.Error())
	}
	return strings.Join(errorMsg, "\n")
}

type clientWrapper struct {
	*storage.Client
	isTesting bool
}

func storeConnect(ctx context.Context) (*clientWrapper, error) {
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
	return &clientWrapper{client, testing}, nil
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

func (c *GcsStore) DownloadSdk(ctx context.Context, name, channel string, target string) error {
	client, err := storeConnect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	var sa = strings.Split(channel, "/")
	if len(sa) != 2 {
		return fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	}
	track, risk := sa[0], sa[1]

	bkt := client.Bucket("sdk-store")
	obj := bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", name, track, risk, name))
	r, err := obj.NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()

	if !osutil.FileExists(target) {
		file, err := os.Create(target)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err = io.Copy(file, r); err != nil {
			return err
		}
	}

	return nil
}

func storeSdkInfoImpl(ctx context.Context, name, channel string) (storeSdk, error) {
	var sSdk storeSdk
	client, err := storeConnect(ctx)
	if err != nil {
		return sSdk, err
	}
	defer client.Close()
	bkt := client.Bucket("sdk-store")

	var sa = strings.Split(channel, "/")
	if len(sa) != 2 {
		return sSdk, fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	}
	track, risk := sa[0], sa[1]
	obj := bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", name, track, risk, name))
	atr, err := obj.Attrs(ctx)
	if err != nil {
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
		sSdk.SdkYAML = fmt.Sprintf(`name: %s
base: ubuntu@22.04`, name)
	}
	return sSdk, nil
}
