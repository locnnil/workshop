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
		Name:      "test-sdk-info",
		PackageID: "U9IBEcXiDkuaPLYjyUBpRZMETAr7g39e",
		Revision:  1,
		Sha3_384:  "cf722fc841c72cf53c4b2db88608589efb173fa2a50837ae6f07597ead85e6e30f36a85e98df9ba78d941b3c9e15ab3d",
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
		Sha3_384:  "cf722fc841c72cf53c4b2db88608589efb173fa2a50837ae6f07597ead85e6e30f36a85e98df9ba78d941b3c9e15ab3d",
	}

	err := client.Download(context.Background(), &buffer, sdk)
	c.Check(err, check.ErrorMatches, `"not-found" SDK \(55555\) not found`)
	c.Check(buffer.Bytes(), check.HasLen, 0)
}
