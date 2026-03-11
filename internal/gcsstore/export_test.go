package gcsstore

import (
	"context"

	"github.com/canonical/workshop/internal/sdk"
)

type (
	StoreSdk  = storeSdk
	SdkReader = sdkReader
)

func FakeSdkStoreInfo(f func(ctx context.Context, name, channel string) (StoreSdk, error)) (restore func()) {
	old := storeSdkInfo
	storeSdkInfo = f
	return func() {
		storeSdkInfo = old
	}
}

func FakeSdkStoreSdkReader(f func(ctx context.Context, setup sdk.Setup) (*SdkReader, error)) (restore func()) {
	old := storeSdkReader
	storeSdkReader = f
	return func() {
		storeSdkReader = old
	}
}
