// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2024 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package sys_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil/sys"
	"github.com/canonical/workshop/internal/testutil"
)

func Test(t *testing.T) { check.TestingT(t) }

type sysSuite struct {
	testutil.BaseTest
}

var _ = check.Suite(&sysSuite{})

func (s *sysSuite) TestLchtimes(c *check.C) {
	root := c.MkDir()

	c.Assert(os.Symlink("target", filepath.Join(root, "link")), check.IsNil)

	// Update atime to +1h and mtime to +1d.
	soon := time.Now().Add(time.Hour)
	tomorrow := soon.Add(24 * time.Hour)
	c.Assert(sys.Lchtimes(filepath.Join(root, "link"), soon, tomorrow), check.IsNil)

	info, err := os.Lstat(filepath.Join(root, "link"))
	c.Assert(err, check.IsNil)
	atime, err := sys.AccessTime(info)
	c.Assert(err, check.IsNil)
	c.Check(atime.Equal(soon), check.Equals, true, check.Commentf("%v != %v", atime, soon))
	mtime := info.ModTime()
	c.Check(mtime.Equal(tomorrow), check.Equals, true, check.Commentf("%v != %v", mtime, tomorrow))

	// Update atime to +2h and leave mtime unchanged.
	soon = soon.Add(time.Hour)
	c.Assert(sys.Lchtimes(filepath.Join(root, "link"), soon, time.Time{}), check.IsNil)

	info, err = os.Lstat(filepath.Join(root, "link"))
	c.Assert(err, check.IsNil)
	atime, err = sys.AccessTime(info)
	c.Assert(err, check.IsNil)
	c.Check(atime.Equal(soon), check.Equals, true, check.Commentf("%v != %v", atime, soon))
	mtime = info.ModTime()
	c.Check(mtime.Equal(tomorrow), check.Equals, true, check.Commentf("%v != %v", mtime, tomorrow))
}
