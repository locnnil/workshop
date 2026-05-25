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

package sdkstore

import (
	"context"
	"encoding/json"

	"gopkg.in/check.v1"
)

type infoIntegration struct{}

var _ = check.Suite(&infoIntegration{})

func (f *infoIntegration) TestSdkInfo(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.info(context.Background(), &response, "test-sdk-info-1")
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkInfoRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *infoIntegration) TestSdkInfoMultiBase(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.info(context.Background(), &response, "test-sdk-info-multi-base-1", WithInfoFields(allInfoFields))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkInfoMultiBaseRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}
