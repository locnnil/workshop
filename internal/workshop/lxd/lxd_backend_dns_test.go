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
	check "gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

type dnsSuite struct{}

var _ = check.Suite(&dnsSuite{})

func (s *dnsSuite) TestGenerateCNAMEOK(c *check.C) {
	cnames := []lxdbackend.CNAME{{
		Workshop:     "dev",
		ProjectId:    "24242424",
		ProjectAlias: "24242424",
	}}
	projects := []workshop.Project{{
		ProjectId: "42424242",
		Path:      "/home/workshop/sdkcraft",
	}}
	entry, err := lxdbackend.GenerateCNAME(cnames, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "sdkcraft",
	})

	cnames = append(cnames, entry)
	entry, err = lxdbackend.GenerateCNAME(cnames, projects, "42424242", "ci")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, lxdbackend.CNAME{
		Workshop:     "ci",
		ProjectId:    "42424242",
		ProjectAlias: "sdkcraft",
	})
}

func (s *dnsSuite) TestGenerateCNAMEBlocksMatchingID(c *check.C) {
	cnames := []lxdbackend.CNAME{{
		Workshop:     "dev",
		ProjectId:    "24242424",
		ProjectAlias: "deadbeef",
	}}
	projects := []workshop.Project{{
		ProjectId: "deadbeef",
		Path:      "/home/workshop/sdkcraft",
	}}
	entry, err := lxdbackend.GenerateCNAME(cnames, projects, "deadbeef", "dev")
	c.Assert(err, check.ErrorMatches, "hostname deadbeef.wp already taken")
	c.Check(entry, check.Equals, lxdbackend.CNAME{})
}

func (s *dnsSuite) TestGenerateCNAMEWithoutProject(c *check.C) {
	entry, err := lxdbackend.GenerateCNAME(nil, nil, "42424242", "dev")
	c.Assert(err, check.ErrorMatches, `project "42424242" not found`)
	c.Check(entry, check.Equals, lxdbackend.CNAME{})
}

func (s *dnsSuite) TestGenerateCNAMEValidation(c *check.C) {
	invalid := lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "42424242",
	}
	replaced := lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "sdk-craft",
	}

	projects := []workshop.Project{{
		ProjectId: "42424242",
		Path:      "/home/workshop/sdk,craft",
	}}
	entry, err := lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, invalid)

	projects[0].Path = "/home/workshop/sdk=craft"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, invalid)

	projects[0].Path = "/home/workshop/sdk#craft"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, invalid)

	projects[0].Path = "/home/workshop/sdk\ncraft"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, invalid)

	projects[0].Path = "/home/workshop/sdk craft"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, replaced)

	projects[0].Path = "/home/workshop/sdk_craft"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, replaced)

	projects[0].Path = "/home/workshop/sdk.craft"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, replaced)

	projects[0].Path = "/home/workshop/café"
	entry, err = lxdbackend.GenerateCNAME(nil, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "xn--caf-dma",
	})
}

func (s *dnsSuite) TestGenerateCNAMETakenByID(c *check.C) {
	cnames := []lxdbackend.CNAME{{
		Workshop:     "dev",
		ProjectId:    "deadbeef",
		ProjectAlias: "sdkcraft",
	}}
	projects := []workshop.Project{{
		ProjectId: "42424242",
		Path:      "/home/workshop/deadBEEF",
	}}
	entry, err := lxdbackend.GenerateCNAME(cnames, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "42424242",
	})
}

func (s *dnsSuite) TestGenerateCNAMETakenByName(c *check.C) {
	cnames := []lxdbackend.CNAME{{
		Workshop:     "dev",
		ProjectId:    "24242424",
		ProjectAlias: "sdkcraft",
	}}
	projects := []workshop.Project{{
		ProjectId: "42424242",
		Path:      "/home/workshop/sdkcraft",
	}}
	entry, err := lxdbackend.GenerateCNAME(cnames, projects, "42424242", "dev")
	c.Assert(err, check.IsNil)
	c.Check(entry, check.Equals, lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "42424242",
	})
}

func (s *dnsSuite) TestMarshalCNAME(c *check.C) {
	cname := lxdbackend.CNAME{
		Workshop:     "dev",
		ProjectId:    "42424242",
		ProjectAlias: "sdkcraft",
	}
	c.Check(cname.String(), check.Equals, "dev.42424242.wp,dev.sdkcraft.wp,dev-42424242.wp,0")

	cname.ProjectAlias = cname.ProjectId
	c.Check(cname.String(), check.Equals, "dev.42424242.wp,dev-42424242.wp,0")
}

func (s *dnsSuite) TestUnmarshalCNAME(c *check.C) {
	var cname lxdbackend.CNAME

	err := cname.UnmarshalText([]byte("dev.42424242.wp,dev.sdkcraft.wp,dev-42424242.wp,0"))
	c.Assert(err, check.IsNil)
	c.Check(cname, check.Equals, lxdbackend.CNAME{Workshop: "dev", ProjectId: "42424242", ProjectAlias: "sdkcraft"})

	err = cname.UnmarshalText([]byte("dev.42424242.wp,dev-42424242.wp,0"))
	c.Assert(err, check.IsNil)
	c.Check(cname, check.Equals, lxdbackend.CNAME{Workshop: "dev", ProjectId: "42424242", ProjectAlias: "42424242"})

	err = cname.UnmarshalText([]byte("dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("a.wp,b.wp,c.wp,d.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.wp,dev.sdkcraft.wp,dev-42424242.wp"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.workshop,dev-42424242.workshop,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.wp,dev42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.wp,dev-4242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.wp,dev-XXXXXXXX.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("ws.42424242.wp,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.4242.wp,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.workshop,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.wp,ws.sdkcraft.wp,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.wp,dev.sdkcraft.workshop,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("ws.42424242.wp,dev.sdkcraft.wp,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.4242.wp,dev.sdkcraft.wp,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)

	err = cname.UnmarshalText([]byte("dev.42424242.workshop,dev.sdkcraft.wp,dev-42424242.wp,0"))
	c.Check(err, check.NotNil)
}
