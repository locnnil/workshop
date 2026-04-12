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
