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
	"bytes"
	"context"
	"crypto/sha3"
	"encoding/hex"

	"gopkg.in/check.v1"
)

type downloadIntegration struct{}

var _ = check.Suite(&downloadIntegration{})

func (f *downloadIntegration) TestDownload(c *check.C) {
	client := NewClient(Config{})
	hash := sha3.New384()
	sdk := SdkArchive{
		Name:      "test-sdk-info-1",
		PackageID: "96TKss360WoMRcFySGMOMhXhwbDdZh0E",
		Revision:  2,
		Sha3_384:  "22e773fd5f052fbfcb077788e5a13b81415ff6f8f7bfd0a65cc19f3ce854054fd1090c04937d420cde5645c2940e29e3",
	}

	err := client.Download(context.Background(), hash, sdk)
	c.Assert(err, check.IsNil)
	digest := hex.EncodeToString(hash.Sum(nil))
	c.Check(digest, check.Equals, sdk.Sha3_384)
}

func (f *downloadIntegration) TestDownloadNotFound(c *check.C) {
	client := NewClient(Config{})
	var buffer bytes.Buffer
	sdk := SdkArchive{
		Name:      "not-found",
		PackageID: "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
		Revision:  55555,
		Sha3_384:  "22e773fd5f052fbfcb077788e5a13b81415ff6f8f7bfd0a65cc19f3ce854054fd1090c04937d420cde5645c2940e29e3",
	}

	err := client.Download(context.Background(), &buffer, sdk)
	c.Check(err, check.ErrorMatches, `"not-found" SDK \(55555\) not found`)
	c.Check(buffer.Bytes(), check.HasLen, 0)
}
