// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package https_test

import (
	"encoding/base64"
	"net/http"
	"strings"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/https"
)

type httpSuite struct{}

var _ = check.Suite(&httpSuite{})

func (s *httpSuite) TestBasicAuthHeader(c *check.C) {
	header := https.BasicAuthHeader("eric", "sekrit")
	c.Assert(len(header), check.Equals, 1)
	auth := header.Get("Authorization")
	fields := strings.Fields(auth)
	c.Assert(len(fields), check.Equals, 2)
	basic, encoded := fields[0], fields[1]
	c.Assert(basic, check.Equals, "Basic")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	c.Assert(err, check.IsNil)
	c.Assert(string(decoded), check.Equals, "eric:sekrit")
}

func (s *httpSuite) TestParseBasicAuthHeader(c *check.C) {
	tests := []struct {
		about          string
		h              http.Header
		expectUserid   string
		expectPassword string
		expectError    string
	}{{
		about:       "no Authorization header",
		h:           http.Header{},
		expectError: "invalid or missing HTTP auth header",
	}, {
		about: "empty Authorization header",
		h: http.Header{
			"Authorization": {""},
		},
		expectError: "invalid or missing HTTP auth header",
	}, {
		about: "Not basic encoding",
		h: http.Header{
			"Authorization": {"NotBasic stuff"},
		},
		expectError: "invalid or missing HTTP auth header",
	}, {
		about: "invalid base64",
		h: http.Header{
			"Authorization": {"Basic not-base64"},
		},
		expectError: "invalid HTTP auth encoding",
	}, {
		about: "no ':'",
		h: http.Header{
			"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("aladdin"))},
		},
		expectError: "invalid HTTP auth contents",
	}, {
		about: "valid credentials",
		h: http.Header{
			"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("aladdin:open sesame"))},
		},
		expectUserid:   "aladdin",
		expectPassword: "open sesame",
	}}
	for i, test := range tests {
		c.Logf("test %d: %s", i, test.about)
		u, p, err := https.ParseBasicAuthHeader(test.h)
		c.Assert(u, check.Equals, test.expectUserid)
		c.Assert(p, check.Equals, test.expectPassword)
		if test.expectError != "" {
			c.Assert(err.Error(), check.Equals, test.expectError)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}
