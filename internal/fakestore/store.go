package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/spf13/afero"
	"google.golang.org/api/option"
)

type StoreClient interface {
	FetchSDK(name, channel, destination string) (SDKFile, error)
}

type SDKFile struct {
	Filename string
	Revision int64
}

func NewStoreClient(fs afero.Fs) (StoreClient, error) {
	return &ObjectStoreClient{Fs: fs}, nil
}

type ObjectStoreClient struct {
	Fs afero.Fs
}

func (c *ObjectStoreClient) FetchSDK(name, channel, destination string) (SDKFile, error) {
	var track, risk string
	var sdk SDKFile

	if sa := strings.Split(channel, "/"); len(sa) != 2 {
		return sdk, fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	} else {
		track, risk = sa[0], sa[1]
	}

	ctx := context.Background()
	if client, err := storage.NewClient(ctx, option.WithoutAuthentication()); err != nil {
		return sdk, err
	} else {
		bkt := client.Bucket("sdk-store")
		var obj *storage.ObjectHandle = bkt.Object(fmt.Sprintf("%s/%s/%s/%s.sdk", name, track, risk, name))
		if atr, err := obj.Attrs(ctx); err != nil {
			return sdk, err
		} else {
			// a simple modulo to keep revision numbers in a readble form for testing
			sdk.Revision = atr.Generation % 1000
		}

		if r, err := obj.NewReader(ctx); err != nil {
			return sdk, err
		} else {
			filename := filepath.Join(destination, fmt.Sprintf("%s_%d.sdk", name, sdk.Revision))
			file, err := c.Fs.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				return sdk, err
			}
			defer r.Close()
			defer file.Close()

			if _, err = io.Copy(file, r); err != nil {
				return sdk, err
			}
			sdk.Filename = filename
		}
	}

	return sdk, nil
}
