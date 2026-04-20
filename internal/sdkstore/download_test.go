// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"bytes"
	"context"
	"crypto/sha3"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"

	"go.uber.org/mock/gomock"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdkstore/path"
	"github.com/canonical/workshop/internal/sdkstore/transport"
	"github.com/canonical/workshop/internal/testutil"
)

type DownloadSuite struct{}

var _ = check.Suite(&DownloadSuite{})

func (s *DownloadSuite) TestDownload(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")
	path := path.MakePath(baseURL)
	data := make([]byte, 1024)

	httpClient := NewMockHTTPClient(ctrl)
	httpClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(r *http.Request) (*http.Response, error) {
		suffix := "/JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy_4242.sdk"
		c.Check(strings.HasSuffix(r.URL.Path, suffix), check.Equals, true, check.Commentf("URL = %q", r.URL))

		return &http.Response{
			StatusCode:    200,
			Body:          io.NopCloser(bytes.NewReader(data)),
			ContentLength: int64(len(data)),
		}, nil
	})

	client := newDownloadClient(path, httpClient)

	hash := sha3.New384()
	sdk := SdkArchive{
		Name:      "test-sdk-download",
		PackageID: "JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy",
		Revision:  4242,
		Sha3_384:  "122237164f723d2f553d519e9f2389145df3a13856ddd72d41b608b8a505d155222455fe868c952104d83f068883e291",
	}

	err := client.Download(context.Background(), hash, sdk)
	c.Assert(err, check.IsNil)
	digest := hex.EncodeToString(hash.Sum(nil))
	c.Check(digest, check.Equals, sdk.Sha3_384)
}

func (s *DownloadSuite) TestDownloadWithReporter(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")
	path := path.MakePath(baseURL)
	size := int64(1024 * 1024)

	httpClient := NewMockHTTPClient(ctrl)
	httpClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(r *http.Request) (*http.Response, error) {
		rng := rand.NewChaCha8([32]byte{})
		return &http.Response{
			StatusCode:    200,
			Body:          io.NopCloser(io.LimitReader(rng, size)),
			ContentLength: size,
		}, nil
	})

	client := newDownloadClient(path, httpClient)

	hash := sha3.New384()
	sdk := SdkArchive{
		Name:      "test-sdk-download",
		PackageID: "JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy",
		Revision:  4242,
		Sha3_384:  "3a5ae5a90e035abd9a1e923a9fa36196d1b4f0da317089ab899083d473fa9ef0778afac4347b804443b849e6170ffede",
	}
	var dones, totals []int64
	reporter := &progress.Reporter{
		Name: "name",
		Report: func(label string, done, total int64) {
			dones = append(dones, done)
			totals = append(totals, total)
		},
	}

	err := client.Download(context.Background(), hash, sdk, WithReporter(reporter))
	c.Assert(err, check.IsNil)
	digest := hex.EncodeToString(hash.Sum(nil))
	c.Check(digest, check.Equals, sdk.Sha3_384)

	c.Check(totals, check.Not(check.HasLen), 0)
	c.Check(dones, check.HasLen, len(totals))
	for _, total := range totals {
		c.Check(total, check.Equals, size)
	}
	minDone := 0
	for _, done := range dones {
		c.Check(int(done), testutil.IntGreaterEqual, minDone)
		c.Check(int(done), testutil.IntLessEqual, int(size))
		minDone = int(done)
	}
}

func (s *DownloadSuite) TestDownloadTruncated(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")
	path := path.MakePath(baseURL)
	data := make([]byte, 512)

	httpClient := NewMockHTTPClient(ctrl)
	httpClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    200,
			Body:          io.NopCloser(bytes.NewReader(data)),
			ContentLength: 1024,
		}, nil
	})

	client := newDownloadClient(path, httpClient)

	sdk := SdkArchive{
		Name:      "test-sdk-download",
		PackageID: "JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy",
		Revision:  4242,
		Sha3_384:  "122237164f723d2f553d519e9f2389145df3a13856ddd72d41b608b8a505d155222455fe868c952104d83f068883e291",
	}

	err := client.Download(context.Background(), io.Discard, sdk)
	c.Check(err, check.ErrorMatches, "downloaded size 512 does not match expected size 1024")
}

func (s *DownloadSuite) TestDownloadInvalidHash(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")
	path := path.MakePath(baseURL)
	data := make([]byte, 1024)

	httpClient := NewMockHTTPClient(ctrl)
	httpClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    200,
			Body:          io.NopCloser(bytes.NewReader(data)),
			ContentLength: int64(len(data)),
		}, nil
	})

	client := newDownloadClient(path, httpClient)

	hash := sha3.New384()
	sdk := SdkArchive{
		Name:      "test-sdk-download",
		PackageID: "JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy",
		Revision:  4242,
		Sha3_384:  "3a5ae5a90e035abd9a1e923a9fa36196d1b4f0da317089ab899083d473fa9ef0778afac4347b804443b849e6170ffede",
	}

	err := client.Download(context.Background(), hash, sdk)
	c.Check(err, check.ErrorMatches, `corrupted download: expected sha3-384 "3a5ae5a90e035abd9a1e923a9fa36196d1b4f0da317089ab899083d473fa9ef0778afac4347b804443b849e6170ffede"`)
}

func (s *DownloadSuite) TestDownloadNotFound(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")
	path := path.MakePath(baseURL)
	data, err := json.Marshal(transport.ErrorResponse{
		ErrorList: []transport.APIError{{
			Code:    "resource-not-found",
			Message: "Cannot download JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy_4242.sdk.",
		}},
	})
	c.Assert(err, check.IsNil)

	httpClient := NewMockHTTPClient(ctrl)
	httpClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewReader(data)),
		}, nil
	})

	client := newDownloadClient(path, httpClient)

	sdk := SdkArchive{
		Name:      "test-sdk-download",
		PackageID: "JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy",
		Revision:  4242,
		Sha3_384:  "122237164f723d2f553d519e9f2389145df3a13856ddd72d41b608b8a505d155222455fe868c952104d83f068883e291",
	}

	err = client.Download(context.Background(), io.Discard, sdk)
	c.Check(err, check.ErrorMatches, `"test-sdk-download" SDK \(4242\) not found`)
}

func (s *DownloadSuite) TestDownloadInternalServerError(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")
	path := path.MakePath(baseURL)

	httpClient := NewMockHTTPClient(ctrl)
	httpClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "500 " + http.StatusText(http.StatusInternalServerError),
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	client := newDownloadClient(path, httpClient)

	sdk := SdkArchive{
		Name:      "test-sdk-download",
		PackageID: "JlcaOWmrknI7ku8GgGUuCrJFoajEPfYy",
		Revision:  4242,
		Sha3_384:  "122237164f723d2f553d519e9f2389145df3a13856ddd72d41b608b8a505d155222455fe868c952104d83f068883e291",
	}

	err := client.Download(context.Background(), io.Discard, sdk)
	c.Check(err, check.ErrorMatches, `SDK download failed: 500 Internal Server Error`)
}
