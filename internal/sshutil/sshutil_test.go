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

//go:build integration

package sshutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sshutil"
)

func Test(t *testing.T) { check.TestingT(t) }

type sshSuite struct{}

var _ = check.Suite(&sshSuite{})

func (s *sshSuite) TestGenerateKey(c *check.C) {
	pub, priv, err := sshutil.GenerateKey("root@dev-42424242.wp")
	c.Assert(err, check.IsNil)

	data, err := priv.MarshalText()
	c.Assert(err, check.IsNil)

	scratch := c.MkDir()
	err = os.WriteFile(filepath.Join(scratch, "id_ed25519"), data, 0600)
	c.Assert(err, check.IsNil)

	cmd := exec.Command("ssh-keygen", "-yf", "id_ed25519")
	cmd.Dir = scratch
	out, err := cmd.Output()
	c.Assert(err, check.IsNil)
	c.Check(string(out), check.Equals, pub.String()+"\n")
}
