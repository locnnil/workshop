// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package https

import (
	"gopkg.in/check.v1"
)

type TLSSuite struct{}

var _ = check.Suite(&TLSSuite{})

func (TLSSuite) TestDisableKeepAlives(c *check.C) {
	transport := DefaultHTTPTransport()
	c.Assert(transport.DisableKeepAlives, check.Equals, false)

	transport = NewHTTPTLSTransport(TransportConfig{})
	c.Assert(transport.DisableKeepAlives, check.Equals, false)

	transport = NewHTTPTLSTransport(TransportConfig{
		DisableKeepAlives: true,
	})
	c.Assert(transport.DisableKeepAlives, check.Equals, true)
}
