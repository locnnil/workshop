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

package lxdbackend_test

import (
	"gopkg.in/check.v1"

	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

func (f *LxdBeTests) TestLaunchProgressReporter(c *check.C) {
	metas := []map[string]any{
		{"download_progress": "metadata: 100% (3.01GB/s)"},
		{"download_progress": "rootfs: 65% (254.2Kb/s)"},
		{"download_progress": "rootfs delta: 65% (254Kb/s)"},
		{"download_progress": "75% (254.2Kb/s)"},
		{"download_progress": "85% (254Kb/s)"},
		{"download_progress": "rootfs: 65 (254Kb/s)"},
		{"download_progress": "rootfs: NaN (254Kb/s)"},
		{"download_progress": 15},
		{"unknown": "rootfs: 65% (254.2Kb/s)"},
		{"download_progress": "65B (254Kb/s)"},
	}

	expected := []*struct {
		label string
		done  int64
		total int64
	}{
		nil,
		{"download", 65, 100},
		{"download", 65, 100},
		{"download", 75, 100},
		{"download", 85, 100},
		nil,
		nil,
		nil,
		nil,
		{"download", 65, 100},
	}

	for i, m := range metas {
		upd := lxdbackend.HandleImageUpdate(m, 100)
		if expected[i] != nil {
			c.Check(upd.Label, check.Equals, expected[i].label)
			c.Check(upd.Done, check.Equals, expected[i].done)
			c.Check(upd.Total, check.Equals, expected[i].total)
		} else {
			c.Check(upd, check.IsNil)
		}
	}
}
