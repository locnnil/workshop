package lxdbackend_test

import (
	"gopkg.in/check.v1"

	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

func (f *LxdBeTests) TestLaunchProgressReporter(c *check.C) {
	metas := []map[string]interface{}{
		{"download_progress": "metadata: 100% (3.01GB/s)"},
		{"download_progress": "rootfs: 65% (254.2Kb/s)"},
		{"download_progress": "rootfs: 65% (254Kb/s)"},
		{"download_progress": "rootfs: 65 (254Kb/s)"},
		{"download_progress": "rootfs: NaN (254Kb/s)"},
		{"download_progress": 15},
		{"unknown": "rootfs: 65% (254.2Kb/s)"},
	}

	expected := []*struct {
		label string
		done  int
		total int
	}{
		nil,
		{"download base image", 65, 100},
		{"download base image", 65, 100},
		nil,
		nil,
		nil,
		nil,
	}

	for i, m := range metas {
		i := i
		reported := false
		checker := func(label string, done, total int) {
			c.Check(label, check.Equals, expected[i].label)
			c.Check(done, check.Equals, expected[i].done)
			c.Check(total, check.Equals, expected[i].total)
			reported = true
		}
		lxdbackend.HandleLaunchUpdate(m, 100, checker)
		if expected[i] != nil {
			c.Assert(reported, check.Equals, true, check.Commentf("No progress was reported on %v, expected: %v", m, expected[i]))
		}
	}
}
