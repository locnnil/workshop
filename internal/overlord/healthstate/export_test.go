// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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

func FakeRetryAttempts(t int) (restore func()) {
	old := retriesAllowed
	retriesAllowed = t
	return func() {
		retriesAllowed = old
	}
}
