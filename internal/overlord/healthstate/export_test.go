package healthstate

import "time"

var (
	KnownStatuses = knownSetHealthStatuses
)

func FakeRetryTimeout(t time.Duration) (restore func()) {
	old := retryTimeout
	retryTimeout = t
	return func() {
		retryTimeout = old
	}
}
