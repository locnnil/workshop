package store

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	Sdks         []*sdk.SdkInfo
	ActionErrors map[string]error
}

type StoreClient interface {
	RetrieveSdk(name, channel, localSdkDir string) (*sdk.SdkInfo, error)
	CheckRefresh(ctx context.Context, sdks []*sdk.SdkInfo) (*StoreResult, error)
}

func NewStoreClient() StoreClient {
	return &ObjectStoreClient{Fs: afero.NewOsFs()}
}

type ObjectStoreClient struct {
	Fs afero.Fs
}

func (c *ObjectStoreClient) CheckRefresh(ctx context.Context, sdks []*sdk.SdkInfo) (*StoreResult, error) {
	if client, err := storage.NewClient(ctx, option.WithoutAuthentication()); err != nil {
		return nil, err
	} else {
		bkt := client.Bucket("sdk-store")
		defer client.Close()

		var result StoreResult
		result.ActionErrors = make(map[string]error)
		result.Sdks = make([]*sdk.SdkInfo, 0)

		for _, s := range sdks {
			var track, risk string
			if sa := strings.Split(s.Channel, "/"); len(sa) != 2 {
				result.ActionErrors[s.Name] = fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", s.Name, s.Channel)
			} else {
				track, risk = sa[0], sa[1]
			}

			var obj *storage.ObjectHandle = bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", s.Name, track, risk, s.Name))
			if atr, err := obj.Attrs(ctx); err != nil {
				result.ActionErrors[s.Name] = err
			} else {
				revision := atr.Generation % 1000
				// if there is a new update, we will reflect it in the return
				// value for this SDK
				if s.Revision != revision {
					result.Sdks = append(result.Sdks, &sdk.SdkInfo{
						Name:     s.Name,
						Channel:  s.Channel,
						Revision: revision,
					})
				} else {
					result.ActionErrors[s.Name] = ErrNoRefreshAvailable
				}
			}

		}
		return &result, nil
	}
}

func (c *ObjectStoreClient) RetrieveSdk(name, channel, localSdkDir string) (*sdk.SdkInfo, error) {
	var track, risk string
	var s sdk.SdkInfo
	var revision int64

	if sa := strings.Split(channel, "/"); len(sa) != 2 {
		return nil, fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	} else {
		track, risk = sa[0], sa[1]
	}

	ctx := context.Background()
	if client, err := storage.NewClient(ctx, option.WithoutAuthentication()); err != nil {
		return &s, err
	} else {
		bkt := client.Bucket("sdk-store")
		defer client.Close()
		var obj *storage.ObjectHandle = bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", name, track, risk, name))
		if atr, err := obj.Attrs(ctx); err != nil {
			return nil, err
		} else {
			/* A simple modulo to keep revision numbers in a readble form for testing */
			revision = atr.Generation % 1000
		}

		if r, err := obj.NewReader(ctx); err != nil {
			return nil, err
		} else {
			defer r.Close()

			s.Name = name
			s.Channel = channel
			s.Revision = revision

			exist, err := afero.Exists(c.Fs, s.Filename())
			if err != nil {
				return nil, err
			}

			if !exist {
				file, err := c.Fs.Create(s.Filename())
				if err != nil {
					return nil, err
				}
				defer file.Close()

				if _, err = io.Copy(file, r); err != nil {
					return nil, err
				}
			}

		}
	}

	return &s, nil
}
