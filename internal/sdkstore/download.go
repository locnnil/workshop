// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"context"
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/canonical/workshop/internal/https"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdkstore/path"
)

// DownloadOption to be passed to Download to customize the resulting request.
type DownloadOption func(*downloadOptions)

type downloadOptions struct {
	reporter *progress.Reporter
}

// WithReporter sets the progress reporter on the option.
func WithReporter(reporter *progress.Reporter) DownloadOption {
	return func(options *downloadOptions) {
		options.reporter = reporter
	}
}

func newDownloadOptions() *downloadOptions {
	return &downloadOptions{}
}

type downloadClient struct {
	path       path.Path
	httpClient https.HTTPClient
}

func newDownloadClient(path path.Path, httpClient https.HTTPClient) *downloadClient {
	return &downloadClient{
		path:       path,
		httpClient: httpClient,
	}
}

type SdkArchive struct {
	Name      string
	PackageID string
	Revision  int
	Sha3_384  string
}

// Download writes the given SDK into the given Writer.
func (c *downloadClient) Download(ctx context.Context, w io.Writer, sdk SdkArchive, options ...DownloadOption) error {
	opts := newDownloadOptions()
	for _, option := range options {
		option(opts)
	}

	r, err := c.download(ctx, sdk)
	if err != nil {
		return err
	}
	defer func() {
		_ = r.Body.Close()
	}()

	hash := sha3.New384()
	writers := []io.Writer{w, hash}
	if opts.reporter != nil {
		rw := &reporterWriter{reporter: opts.reporter, total: r.ContentLength}
		writers = append(writers, rw)
	}

	size, err := io.Copy(io.MultiWriter(writers...), r.Body)
	if err != nil {
		return err
	}
	if size != r.ContentLength {
		return fmt.Errorf("downloaded size %d does not match expected size %d", size, r.ContentLength)
	}

	digest := hex.EncodeToString(hash.Sum(nil))
	if digest != sdk.Sha3_384 {
		return fmt.Errorf("corrupted download: expected sha3-384 %q", sdk.Sha3_384)
	}
	return nil
}

func (c *downloadClient) download(ctx context.Context, sdk SdkArchive) (resp *http.Response, err error) {
	path := c.path.JoinPath(fmt.Sprintf("%s_%v.sdk", sdk.PackageID, sdk.Revision))
	req, err := http.NewRequestWithContext(ctx, "GET", path.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot make new request: %w", err)
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return nil, err
	}
	// If we get anything but a 200 status code, we don't know how to correctly
	// handle that scenario. Return early and deal with the failure later on.
	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	logger.Noticef("On download from %q: response code %s", path, resp.Status)

	// Ensure we drain the response body so this connection can be reused. As
	// there is no error message, we have no ability other than to check the
	// status codes.
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%q SDK (%v) not found", sdk.Name, sdk.Revision)
	}

	// Server error, nothing we can do other than inform the user that the
	// archive was unavailable.
	return nil, fmt.Errorf("SDK download failed: %s", resp.Status)
}

type reporterWriter struct {
	reporter *progress.Reporter
	done     int64
	total    int64
}

func (r *reporterWriter) Write(p []byte) (n int, err error) {
	r.done += int64(len(p))
	r.reporter.Report("download", r.done, r.total)
	return len(p), nil
}
