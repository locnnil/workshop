package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

type StoreClient interface {
	FetchSDK(name, channel, destination string) (string, error)
}

func NewStoreClient(fs afero.Fs) (StoreClient, error) {
	return &ObjectStoreClient{Fs: fs}, nil
}

type ObjectStoreClient struct {
	Fs afero.Fs
}

func (c *ObjectStoreClient) FetchSDK(name, channel, destination string) (string, error) {
	var track, risk string
	if sa := strings.Split(channel, "/"); len(sa) != 2 {
		return "", fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	} else {
		track, risk = sa[0], sa[1]
	}

	filename := filepath.Join(destination, fmt.Sprintf("%s_%s_%s.sdk", name, track, risk))
	file, err := c.Fs.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}

	defer file.Close()
	return filename, nil
}
