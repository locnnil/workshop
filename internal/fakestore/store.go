package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	util "github.com/canonical/workspace/internal"
	"github.com/spf13/afero"
	"google.golang.org/api/option"
)

type StoreClient interface {
	RetrieveSdk(name, channel string) (SdkBlob, error)
}

type SdkBlob struct {
	Name     string
	Channel  string
	Revision int64
}

func ToSdkFilename(name string, revision int64) string {
	return filepath.Join(util.SdksDir, fmt.Sprintf("%s_%d.sdk", name, revision))
}

func NewStoreClient() (StoreClient, error) {
	return &ObjectStoreClient{Fs: afero.NewOsFs()}, nil
}

type ObjectStoreClient struct {
	Fs afero.Fs
}

func (c *ObjectStoreClient) RetrieveSdk(name, channel string) (SdkBlob, error) {
	var track, risk string
	var sdk SdkBlob
	var revision int64

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
			/* A simple modulo to keep revision numbers in a readble form for testing */
			revision = atr.Generation % 1000
		}

		if r, err := obj.NewReader(ctx); err != nil {
			return sdk, err
		} else {
			defer r.Close()

			filename := ToSdkFilename(name, revision)

			file, err := c.Fs.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0600)
			if err != nil && !os.IsExist(err) {
				return sdk, err
			} else if os.IsExist(err) {
				/* Reuse the existing blob if present */
				sdk.Name = name
				sdk.Channel = channel
				sdk.Revision = revision
				return sdk, nil
			}
			defer file.Close()

			if _, err = io.Copy(file, r); err != nil {
				return sdk, err
			}
			sdk.Name = name
			sdk.Channel = channel
			sdk.Revision = revision
		}
	}

	return sdk, nil
}
