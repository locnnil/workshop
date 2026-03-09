// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package https

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"time"

	"go.uber.org/mock/gomock"
	"gopkg.in/check.v1"
)

type clientSuite struct{}

var _ = check.Suite(&clientSuite{})

func (s *clientSuite) TestNewClient(c *check.C) {
	client := NewClient()
	c.Assert(client, check.NotNil)
}

type httpSuite struct {
	server *httptest.Server
}

var _ = check.Suite(&httpSuite{})

func (s *httpSuite) SetUpTest(c *check.C) {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
}

func (s *httpSuite) TearDownTest(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *httpSuite) TestInsecureClientAllowAccess(c *check.C) {
	client := NewClient(WithSkipHostnameVerification(true))
	resp, err := client.Get(context.Background(), s.server.URL)
	c.Assert(err, check.IsNil)
	_ = resp.Body.Close()
}

func (s *httpSuite) TestSecureClientAllowAccess(c *check.C) {
	client := NewClient()
	resp, err := client.Get(context.Background(), s.server.URL)
	c.Assert(err, check.IsNil)
	_ = resp.Body.Close()
}

// NewClient with a default config used to overwrite http.DefaultClient.Jar
// field; add a regression test for that.
func (s *httpSuite) TestDefaultClientJarNotOverwritten(c *check.C) {
	oldJar := http.DefaultClient.Jar

	jar, err := cookiejar.New(nil)
	c.Assert(err, check.IsNil)

	client := NewClient(WithCookieJar(jar))

	hc := client.HTTPClient.(*http.Client)
	c.Assert(hc.Jar, check.Equals, jar)
	c.Assert(http.DefaultClient.Jar, check.Not(check.Equals), jar)
	c.Assert(http.DefaultClient.Jar, check.Equals, oldJar)

	http.DefaultClient.Jar = oldJar
}

func (s *httpSuite) TestRequestRecorder(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	dummyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, _ = fmt.Fprintln(res, "they are listening...")
	}))
	defer dummyServer.Close()

	validTarget := fmt.Sprintf("%s/tin/foil", dummyServer.URL)
	validTargetURL, err := url.Parse(validTarget)
	c.Assert(err, check.IsNil)

	invalidTarget := "btc://secret/wallet"
	invalidTargetURL, err := url.Parse(invalidTarget)
	c.Assert(err, check.IsNil)

	recorder := NewMockRequestRecorder(ctrl)
	recorder.EXPECT().Record("GET", validTargetURL, gomock.AssignableToTypeOf(&http.Response{}), gomock.AssignableToTypeOf(time.Duration(42)))
	recorder.EXPECT().RecordError("PUT", invalidTargetURL, gomock.Any())

	ctx := context.Background()
	client := NewClient(WithRequestRecorder(recorder))
	res, err := client.Get(ctx, validTarget)
	c.Assert(err, check.IsNil)
	defer res.Body.Close()

	req, err := http.NewRequestWithContext(ctx, "PUT", invalidTarget, nil)
	c.Assert(err, check.IsNil)
	_, err = client.Do(req) //nolint:bodyclose
	c.Assert(err, check.NotNil)
}

func (s *httpSuite) TestRetry(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	attempts := 0
	retries := 3
	dummyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if attempts < retries-1 {
			res.WriteHeader(http.StatusBadGateway)
		} else {
			res.WriteHeader(http.StatusOK)
		}
		attempts++
		_, _ = fmt.Fprintln(res, "they are listening...")
	}))
	defer dummyServer.Close()

	validTarget := fmt.Sprintf("%s/tin/foil", dummyServer.URL)
	validTargetURL, err := url.Parse(validTarget)
	c.Assert(err, check.IsNil)

	recorder := NewMockRequestRecorder(ctrl)
	recorder.EXPECT().Record("GET", validTargetURL, gomock.AssignableToTypeOf(&http.Response{}), gomock.AssignableToTypeOf(time.Duration(42))).Times(retries)

	client := NewClient(
		// We can use the request recorder to monitor how many retries have been
		// made.
		WithRequestRecorder(recorder),
		WithRequestRetrier(RetryPolicy{
			Delay:    time.Nanosecond,
			Attempts: retries,
			MaxDelay: time.Minute,
		}),
	)
	res, err := client.Get(context.Background(), validTarget)
	c.Assert(err, check.IsNil)
	defer res.Body.Close()
}

func (s *httpSuite) TestRetryExceeded(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	retries := 3
	dummyServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintln(res, "they are listening...")
	}))
	defer dummyServer.Close()

	validTarget := fmt.Sprintf("%s/tin/foil", dummyServer.URL)
	validTargetURL, err := url.Parse(validTarget)
	c.Assert(err, check.IsNil)

	recorder := NewMockRequestRecorder(ctrl)
	recorder.EXPECT().Record("GET", validTargetURL, gomock.AssignableToTypeOf(&http.Response{}), gomock.AssignableToTypeOf(time.Duration(42))).Times(retries)

	client := NewClient(
		// We can use the request recorder to monitor how many retries have been
		// made.
		WithRequestRecorder(recorder),
		WithRequestRetrier(RetryPolicy{
			Delay:    time.Nanosecond,
			Attempts: retries,
			MaxDelay: time.Minute,
		}),
	)
	_, err = client.Get(context.Background(), validTarget) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `.*attempt count exceeded: retryable error`)
}

type httpTLSServerSuite struct {
	server *httptest.Server
}

var _ = check.Suite(&httpTLSServerSuite{})

func (s *httpTLSServerSuite) SetUpTest(c *check.C) {
	// NewTLSServer returns a server which serves TLS, but
	// its certificates are not validated by the default
	// OS certificates, so any HTTPS request will fail
	// unless a non-validating client is used.
	s.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
}

func (s *httpTLSServerSuite) TearDownTest(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *httpTLSServerSuite) TestValidatingClientGetter(c *check.C) {
	client := NewClient()
	_, err := client.Get(context.Background(), s.server.URL) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, "(.|\n)*x509: certificate signed by unknown authority")
}

func (s *httpTLSServerSuite) TestNonValidatingClientGetter(c *check.C) {
	client := NewClient(WithSkipHostnameVerification(true))
	resp, err := client.Get(context.Background(), s.server.URL)
	c.Assert(err, check.IsNil)
	_ = resp.Body.Close()
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *httpTLSServerSuite) TestGetHTTPClientWithCertsVerify(c *check.C) {
	s.testGetHTTPClientWithCerts(c, true)
}

func (s *httpTLSServerSuite) TestGetHTTPClientWithCertsNoVerify(c *check.C) {
	s.testGetHTTPClientWithCerts(c, false)
}

func (s *httpTLSServerSuite) testGetHTTPClientWithCerts(c *check.C, skip bool) {
	caPEM := new(bytes.Buffer)
	err := pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: s.server.Certificate().Raw,
	})
	c.Assert(err, check.IsNil)

	client := NewClient(
		WithCACertificates(caPEM.String()),
		WithSkipHostnameVerification(skip),
	)
	resp, err := client.Get(context.Background(), s.server.URL)
	c.Assert(err, check.IsNil)
	c.Assert(resp.Body.Close(), check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *clientSuite) TestDisableKeepAlives(c *check.C) {
	client := NewClient()
	hc := client.HTTPClient.(*http.Client)
	transport := hc.Transport.(*http.Transport)
	c.Assert(transport.DisableKeepAlives, check.Equals, false)

	client = NewClient(WithDisableKeepAlives(false))
	hc = client.HTTPClient.(*http.Client)
	transport = hc.Transport.(*http.Transport)
	c.Assert(transport.DisableKeepAlives, check.Equals, false)

	client = NewClient(WithDisableKeepAlives(true))
	hc = client.HTTPClient.(*http.Client)
	transport = hc.Transport.(*http.Transport)
	c.Assert(transport.DisableKeepAlives, check.Equals, true)
}
