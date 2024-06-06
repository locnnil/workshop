package store

import (
	"context"
)

type StoreSdk = storeSdk

func FakeSdkStoreInfo(f func(ctx context.Context, name, channel string) (storeSdk, error)) (restore func()) {
	old := f
	storeSdkInfo = f
	return func() {
		storeSdkInfo = old
	}
}
