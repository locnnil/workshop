// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package path

import (
	"net/url"
	"testing"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

func MustParseURL(c *check.C, path string) *url.URL {
	u, err := url.Parse(path)
	c.Assert(err, check.IsNil)
	return u
}
