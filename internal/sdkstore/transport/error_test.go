// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package transport

import (
	"encoding/json"
	"testing"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ErrorSuite struct{}

var _ = check.Suite(&ErrorSuite{})

func (s *ErrorSuite) TestNoErrors(c *check.C) {
	var errors APIErrors
	err := errors.Error()
	c.Assert(err, check.DeepEquals, "")
}

func (s *ErrorSuite) TestNoErrorsWithEmptySlice(c *check.C) {
	errors := make(APIErrors, 0)
	err := errors.Error()
	c.Assert(err, check.DeepEquals, "")
}

func (s *ErrorSuite) TestWithOneError(c *check.C) {
	errors := APIErrors{{
		Message: "one",
	}}
	err := errors.Error()
	c.Assert(err, check.DeepEquals, `one`)
}

func (s *ErrorSuite) TestWithMultipleErrors(c *check.C) {
	errors := APIErrors{
		{Message: "one"},
		{Message: "two"},
	}
	err := errors.Error()
	c.Assert(err, check.DeepEquals, `one
two`)
}

func (s *ErrorSuite) TestExtras(c *check.C) {
	expected := APIError{
		Extra: APIErrorExtra{
			Releases: []Release{
				{Architecture: "amd64", Channel: "22.04"},
			},
		},
	}
	bytes, err := json.Marshal(expected)
	c.Assert(err, check.IsNil)

	var result APIError
	err = json.Unmarshal(bytes, &result)
	c.Assert(err, check.IsNil)

	c.Assert(result, check.DeepEquals, expected)
}
