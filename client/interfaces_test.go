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

package client_test

import (
	"fmt"
	"testing"

	"github.com/canonical/workshop/client"
)

func FuzzRemount(f *testing.F) {
	testcases := []string{"albert/go:plug", "albert-test/go-sdk:plug7", "albert-test/go-sdk_:a-plug", "work_shop/go-sdk_:a-plug"}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, a string) {
		ref, err := client.ParseShortPlugRef(a)
		if err == nil {
			if fmt.Sprintf("%s/%s:%s", ref.Workshop, ref.Sdk, ref.Name) != a {
				t.Errorf("plug %s cannot be reverted to a PlugRef after parsing", a)
			}
		}

		if err != nil {
			if err.Error() != fmt.Sprintf("cannot remount: unknown plug reference %s", a) {
				t.Errorf("unknown error returned: %v", err)
			}
		}
	})
}
