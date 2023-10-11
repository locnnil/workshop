package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/spf13/afero"
	"google.golang.org/api/option"
)

var (
	ErrNoRefreshAvailable = errors.New("SDK has no update available")
)

type StoreAction int

const (
	Refresh StoreAction = iota
)

func (s StoreAction) String() string {
	return [...]string{"refresh"}[s]
}

type StoreResult struct {
	Sdks         []*sdk.Info
	ActionErrors map[string]error
}

type StoreClient interface {
	RetrieveSdk(name, channel, localSdkDir string) (sdk.Setup, error)
}

func NewStoreClient() StoreClient {
	return &ObjectStoreClient{Fs: afero.NewOsFs()}
}

type ObjectStoreClient struct {
	Fs afero.Fs
}

func storeConnect() (*storage.Client, error) {
	if url := os.Getenv("SDK_STORE_URL"); url != "" {
		// Set STORAGE_EMULATOR_HOST environment variable for GSC.
		err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:9000")
		if err != nil {
			return nil, err
		}
		client, err := storage.NewClient(context.Background(),
			option.WithEndpoint(url))
		return client, err

	}
	client, err := storage.NewClient(context.Background(), option.WithoutAuthentication())
	return client, err
}

func (c *ObjectStoreClient) RetrieveSdk(name, channel, localSdkDir string) (sdk.Setup, error) {
	var track, risk string
	var s sdk.Setup
	var revision int64

	if sa := strings.Split(channel, "/"); len(sa) != 2 {
		return s, fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	} else {
		track, risk = sa[0], sa[1]
	}

	ctx := context.Background()
	if client, err := storeConnect(); err != nil {
		return s, err
	} else {
		bkt := client.Bucket("sdk-store")
		defer client.Close()
		var obj *storage.ObjectHandle = bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", name, track, risk, name))
		if atr, err := obj.Attrs(ctx); err != nil {
			return s, err
		} else {
			/* A simple modulo to keep revision numbers in a readble form for testing */
			revision = atr.Generation % 1000
		}

		if r, err := obj.NewReader(ctx); err != nil {
			return s, err
		} else {
			defer r.Close()

			s.Name = name
			s.Channel = channel
			s.Revision = revision

			exist, err := afero.Exists(c.Fs, s.Filename())
			if err != nil {
				return s, err
			}

			if !exist {
				file, err := c.Fs.Create(s.Filename())
				if err != nil {
					return s, err
				}
				defer file.Close()

				if _, err = io.Copy(file, r); err != nil {
					return s, err
				}
			}

		}
	}

	return s, nil
}
