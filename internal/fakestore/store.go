package store

import "io"

type StoreClient interface {
	FetchSDK(name, channel string) (io.Reader, error)
}

type ObjectStoreClient struct {
}

func (c *ObjectStoreClient) FetchSDK(name, channel string) (io.Reader, error) {
	return nil, nil
}
