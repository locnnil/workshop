// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3.
package https

import (
	"context"
	"net/http"
	"time"

	"github.com/juju/clock"
	"go.uber.org/mock/gomock"
	"gopkg.in/check.v1"
)

type DialContextMiddlewareSuite struct{}

var _ = check.Suite(&DialContextMiddlewareSuite{})

var isLocalAddrTests = []struct {
	addr    string
	isLocal bool
}{
	{addr: "localhost:456", isLocal: true},
	{addr: "127.0.0.1:1234", isLocal: true},
	{addr: "[::1]:4567", isLocal: true},
	{addr: "localhost:smtp", isLocal: true},
	{addr: "123.45.67.5", isLocal: false},
	{addr: "0.1.2.3", isLocal: false},
	{addr: "10.0.43.6:12345", isLocal: false},
	{addr: ":456", isLocal: false},
	{addr: "12xz4.5.6", isLocal: false},
}

func (s *DialContextMiddlewareSuite) TestIsLocalAddr(c *check.C) {
	for i, test := range isLocalAddrTests {
		c.Logf("test %d: %v", i, test.addr)
		c.Assert(isLocalAddr(test.addr), check.Equals, test.isLocal)
	}
}

func (s *DialContextMiddlewareSuite) TestInsecureClientNoAccess(c *check.C) {
	client := NewClient(
		WithTransportMiddlewares(
			DialContextMiddleware(NewLocalDialBreaker(false)),
		),
		WithSkipHostnameVerification(true),
	)
	_, err := client.Get(context.Background(), "http://0.1.2.3:1234") //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `.*access to address "0.1.2.3:1234" not allowed`)
}

func (s *DialContextMiddlewareSuite) TestSecureClientNoAccess(c *check.C) {
	client := NewClient(
		WithTransportMiddlewares(
			DialContextMiddleware(NewLocalDialBreaker(false)),
		),
	)
	_, err := client.Get(context.Background(), "http://0.1.2.3:1234") //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `.*access to address "0.1.2.3:1234" not allowed`)
}

type LocalDialBreakerSuite struct{}

var _ = check.Suite(&LocalDialBreakerSuite{})

func (s *LocalDialBreakerSuite) TestAllowed(c *check.C) {
	breaker := NewLocalDialBreaker(true)

	for i, test := range isLocalAddrTests {
		c.Logf("test %d: %v", i, test.addr)
		allowed := breaker.Allowed(test.addr)
		c.Assert(allowed, check.Equals, true)
	}
}

func (s *LocalDialBreakerSuite) TestLocalAllowed(c *check.C) {
	breaker := NewLocalDialBreaker(false)

	for i, test := range isLocalAddrTests {
		c.Logf("test %d: %v", i, test.addr)
		allowed := breaker.Allowed(test.addr)
		c.Assert(allowed, check.Equals, test.isLocal)
	}
}

func (s *LocalDialBreakerSuite) TestLocalAllowedAfterTrip(c *check.C) {
	breaker := NewLocalDialBreaker(true)

	for i, test := range isLocalAddrTests {
		c.Logf("test %d: %v", i, test.addr)
		allowed := breaker.Allowed(test.addr)
		c.Assert(allowed, check.Equals, true)

		breaker.Trip()

		allowed = breaker.Allowed(test.addr)
		c.Assert(allowed, check.Equals, test.isLocal)

		// Reset the breaker.
		breaker.Trip()
	}
}

type RetrySuite struct{}

var _ = check.Suite(&RetrySuite{})

type timeoutError struct{}

func (timeoutError) Error() string {
	return "net/http: TLS handshake timeout"
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Temporary() bool {
	return true
}

func (s *RetrySuite) TestRetryNotRequired(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: 3,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock.WallClock)

	resp, err := middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *RetrySuite) TestRetryRequired(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusBadGateway,
	}, nil).Times(2)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(gomock.Any()).Return(ch).AnyTimes()

	retries := 3
	go func() {
		for range retries {
			ch <- time.Now()
		}
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	resp, err := middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *RetrySuite) TestRetryRequiredForSafeMethodNetworkTimeout(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(nil, timeoutError{}).Times(2)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(gomock.Any()).Return(ch).AnyTimes()

	retries := 3
	go func() {
		for range retries {
			ch <- time.Now()
		}
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	resp, err := middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *RetrySuite) TestRetryNotRequiredForUnsafeMethodNetworkTimeout(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("POST", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(nil, timeoutError{})

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now())

	retries := 3
	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	_, err = middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `net/http: TLS handshake timeout`)
}

func (s *RetrySuite) TestRetryRequiredUsingBackoff(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	header := make(http.Header)
	header.Add("Retry-After", "42")

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     header,
	}, nil).Times(2)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(time.Second * 42).Return(ch).Times(2)

	retries := 3
	go func() {
		for range retries {
			ch <- time.Now()
		}
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	resp, err := middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *RetrySuite) TestRetryRequiredUsingBackoffDate(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	header := make(http.Header)
	header.Add("Retry-After", "Wed, 21 Oct 2015 07:28:00 UTC")

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     header,
	}, nil).Times(2)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	now, err := time.Parse(time.RFC1123, "Wed, 21 Oct 2015 07:27:18 UTC")
	c.Assert(err, check.IsNil)
	clock.EXPECT().Now().Return(now).AnyTimes()
	clock.EXPECT().After(time.Second * 42).Return(ch).Times(2)

	retries := 3
	go func() {
		for range retries {
			ch <- now
		}
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	resp, err := middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusOK)
}

func (s *RetrySuite) TestRetryRequiredUsingBackoffFailure(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	header := make(http.Header)
	header.Add("Retry-After", "2520")

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     header,
	}, nil)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(time.Minute * 42).Return(ch)

	retries := 3
	go func() {
		ch <- time.Now()
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Minute,
		MaxDelay: time.Second,
	}, clock)

	_, err = middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `API request retry is not accepting further requests until .*`)
}

func (s *RetrySuite) TestRetryRequiredUsingBackoffError(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	header := make(http.Header)
	header.Add("Retry-After", "!@1234391asd--\\123")

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     header,
	}, nil)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(time.Minute * 1).Return(ch)

	retries := 3
	go func() {
		ch <- time.Now()
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Minute,
		MaxDelay: time.Second,
	}, clock)

	_, err = middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `API request retry is not accepting further requests until .*`)
}

func (s *RetrySuite) TestRetryRequiredAndExceeded(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(&http.Response{
		StatusCode: http.StatusBadGateway,
	}, nil).Times(3)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(gomock.Any()).Return(ch).AnyTimes()

	retries := 3
	go func() {
		for range retries {
			ch <- time.Now()
		}
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	_, err = middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `attempt count exceeded: retryable error`)
}

func (s *RetrySuite) TestRetryRequiredForNetworkTimeoutAndExceeded(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	req, err := http.NewRequest("GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)
	transport.EXPECT().RoundTrip(req).Return(nil, timeoutError{}).Times(3)

	ch := make(chan time.Time)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now()).AnyTimes()
	clock.EXPECT().After(gomock.Any()).Return(ch).AnyTimes()

	retries := 3
	go func() {
		for range retries {
			ch <- time.Now()
		}
	}()

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: retries,
		Delay:    time.Second,
		MaxDelay: time.Minute,
	}, clock)

	_, err = middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `attempt count exceeded: net/http: TLS handshake timeout`)
}

func (s *RetrySuite) TestRetryRequiredContextKilled(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, "GET", "http://meshuggah.rocks", nil)
	c.Assert(err, check.IsNil)

	transport := NewMockRoundTripper(ctrl)

	clock := NewMockClock(ctrl)
	clock.EXPECT().Now().Return(time.Now())

	middleware := makeRetryMiddleware(transport, RetryPolicy{
		Attempts: 3,
		Delay:    time.Second,
	}, clock)

	// Nothing should run, the context has been cancelled.
	cancel()

	_, err = middleware.RoundTrip(req) //nolint:bodyclose
	c.Assert(err, check.ErrorMatches, `context canceled`)
}
