// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdkstore/transport"
)

type ErrorsSuite struct{}

var _ = check.Suite(&ErrorsSuite{})

func (s *ErrorsSuite) TestHandleBasicAPIErrors(c *check.C) {
	var list transport.APIErrors
	err := handleBasicAPIErrors(list)
	c.Assert(err, check.IsNil)
}

func (s *ErrorsSuite) TestHandleBasicAPIErrorsNotFound(c *check.C) {
	list := transport.APIErrors{{Code: transport.ErrorCodeNotFound, Message: "foo"}}
	err := handleBasicAPIErrors(list)
	c.Assert(err, check.ErrorMatches, `SDK not found`)
}

func (s *ErrorsSuite) TestHandleBasicAPIErrorsOther(c *check.C) {
	list := transport.APIErrors{{Code: "other", Message: "foo"}}
	err := handleBasicAPIErrors(list)
	c.Assert(err, check.ErrorMatches, `foo`)
}
