// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package path

import (
	"gopkg.in/check.v1"
)

type PathSuite struct{}

var _ = check.Suite(&PathSuite{})

func (s *PathSuite) TestJoinPath(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path/")

	path := MakePath(rawURL)
	appPath := path.JoinPath("entity", "app")

	c.Assert(appPath.String(), check.Equals, "http://foobar/v1/path/entity/app")
}

func (s *PathSuite) TestJoinPathMultipleTimes(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path/")

	path := MakePath(rawURL)
	entityPath := path.JoinPath("entity")
	appPath := entityPath.JoinPath("app")

	c.Assert(appPath.String(), check.Equals, "http://foobar/v1/path/entity/app")
}

func (s *PathSuite) TestJoinPathEscape(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path/")

	path := MakePath(rawURL)
	entityPath := path.JoinPath("en/ti/ty")
	appPath := entityPath.JoinPath("?app%")

	c.Assert(appPath.String(), check.Equals, "http://foobar/v1/path/en%2Fti%2Fty/%3Fapp%25")
}

func (s *PathSuite) TestQuery(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path")

	path := MakePath(rawURL)

	newPath, err := path.Query("q", "foo")
	c.Assert(err, check.IsNil)
	c.Assert(path.String(), check.Equals, "http://foobar/v1/path")
	c.Assert(newPath.String(), check.Equals, "http://foobar/v1/path?q=foo")
}

func (s *PathSuite) TestQueryEmptyValue(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path")

	path := MakePath(rawURL)

	newPath, err := path.Query("q", "")
	c.Assert(err, check.IsNil)
	c.Assert(path.String(), check.Equals, newPath.String())
	c.Assert(newPath.String(), check.Equals, "http://foobar/v1/path")
}

func (s *PathSuite) TestJoinQuery(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path")

	path := MakePath(rawURL)
	entityPath := path.JoinPath("entity")
	appPath := entityPath.JoinPath("app")

	newPath, err := appPath.Query("q", "foo")
	c.Assert(err, check.IsNil)
	c.Assert(appPath.String(), check.Equals, "http://foobar/v1/path/entity/app")
	c.Assert(newPath.String(), check.Equals, "http://foobar/v1/path/entity/app?q=foo")
}

func (s *PathSuite) TestMultipleQueries(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path")

	path := MakePath(rawURL)

	newPath, err := path.Query("q", "foo1")
	c.Assert(err, check.IsNil)

	newPath, err = newPath.Query("q", "foo2")
	c.Assert(err, check.IsNil)

	newPath, err = newPath.Query("x", "bar")
	c.Assert(err, check.IsNil)

	c.Assert(path.String(), check.Equals, "http://foobar/v1/path")
	c.Assert(newPath.String(), check.Equals, "http://foobar/v1/path?q=foo1&q=foo2&x=bar")
}

func (s *PathSuite) TestQueries(c *check.C) {
	rawURL := MustParseURL(c, "http://foobar/v1/path")

	path := MakePath(rawURL)

	newPath0, err := path.Query("a", "foo1")
	c.Assert(err, check.IsNil)
	c.Assert(newPath0.String(), check.Equals, "http://foobar/v1/path?a=foo1")

	newPath1, err := newPath0.Query("b", "foo2")
	c.Assert(err, check.IsNil)
	c.Assert(newPath1.String(), check.Equals, "http://foobar/v1/path?a=foo1&b=foo2")

	newPath1, err = newPath1.Query("c", "foo3")
	c.Assert(err, check.IsNil)
	c.Assert(newPath1.String(), check.Equals, "http://foobar/v1/path?a=foo1&b=foo2&c=foo3")
}
