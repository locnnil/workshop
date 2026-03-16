// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"net/url"

	"gopkg.in/check.v1"
)

type ConfigSuite struct{}

var _ = check.Suite(&ConfigSuite{})

func (s *ConfigSuite) TestBasePath(c *check.C) {
	u, err := url.Parse("http://api.foo.bar.com")
	c.Assert(err, check.IsNil)
	path := basePath(u)
	c.Assert(path.String(), check.Equals, "http://api.foo.bar.com/v2/sdks")
}

func (s *ConfigSuite) TestBasePathWithTrailingSlash(c *check.C) {
	u, err := url.Parse("http://api.foo.bar.com/")
	c.Assert(err, check.IsNil)
	path := basePath(u)
	c.Assert(path.String(), check.Equals, "http://api.foo.bar.com/v2/sdks")
}
