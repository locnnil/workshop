package store

import (
	"context"
	"io"

	"github.com/canonical/workshop/internal/sdk"
)

type (
	StoreSdk = storeSdk
)

func FakeSdkStoreInfo(f func(ctx context.Context, name, channel string) (storeSdk, error)) (restore func()) {
	old := storeSdkInfo
	storeSdkInfo = f
	return func() {
		storeSdkInfo = old
	}
}

func FakeSdkStoreSdkReader(f func(ctx context.Context, setup sdk.Setup) (io.ReadCloser, error)) (restore func()) {
	old := storeSdkReader
	storeSdkReader = f
	return func() {
		storeSdkReader = old
	}
}
