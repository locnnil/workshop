// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package https

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	"github.com/juju/clock"
)

// Option to be passed into the transport construction to customize the
// default transport.
type Option func(*options)

type options struct {
	caCertificates           []string
	cookieJar                http.CookieJar
	disableKeepAlives        bool
	skipHostnameVerification bool
	tlsHandshakeTimeout      time.Duration
	middlewares              []TransportMiddleware
	httpClient               *http.Client
	requestRecorder          RequestRecorder
	retryPolicy              *RetryPolicy
}

// WithCACertificates contains Authority certificates to be used to validate
// certificates of cloud infrastructure components.
// The contents are Base64 encoded x.509 certs.
func WithCACertificates(value ...string) Option {
	return func(opt *options) {
		opt.caCertificates = value
	}
}

// WithCookieJar is used to insert relevant cookies into every
// outbound Request and is updated with the cookie values
// of every inbound Response. The Jar is consulted for every
// redirect that the Client follows.
//
// If Jar is nil, cookies are only sent if they are explicitly
// set on the Request.
func WithCookieJar(value http.CookieJar) Option {
	return func(opt *options) {
		opt.cookieJar = value
	}
}

// WithDisableKeepAlives will disable HTTP keep alives, not TCP keep alives.
// Disabling HTTP keep alives will only use the connection to the server for a
// single HTTP request, slowing down subsequent requests and creating a lot of
// garbage for the collector.
func WithDisableKeepAlives(value bool) Option {
	return func(opt *options) {
		opt.disableKeepAlives = value
	}
}

// WithSkipHostnameVerification will skip hostname verification on the TLS/SSL
// certificates.
func WithSkipHostnameVerification(value bool) Option {
	return func(opt *options) {
		opt.skipHostnameVerification = value
	}
}

// WithTLSHandshakeTimeout will modify how long a TLS handshake should take.
// Setting the value to zero will mean that no timeout will occur.
func WithTLSHandshakeTimeout(value time.Duration) Option {
	return func(opt *options) {
		opt.tlsHandshakeTimeout = value
	}
}

// WithTransportMiddlewares allows the wrapping or modification of the existing
// transport for a given client.
// In an ideal world, all transports should be cloned to prevent the
// modification of an existing client transport.
func WithTransportMiddlewares(middlewares ...TransportMiddleware) Option {
	return func(opt *options) {
		opt.middlewares = middlewares
	}
}

// WithHTTPClient allows to define the http.Client to use.
func WithHTTPClient(value *http.Client) Option {
	return func(opt *options) {
		opt.httpClient = value
	}
}

// WithRequestRecorder specifies a RequestRecorder used for recording outgoing
// http requests regardless of whether they succeeded or failed.
func WithRequestRecorder(value RequestRecorder) Option {
	return func(opt *options) {
		opt.requestRecorder = value
	}
}

// WithRequestRetrier specifies a request retrying policy.
func WithRequestRetrier(value RetryPolicy) Option {
	return func(opt *options) {
		opt.retryPolicy = &value
	}
}

// Create a options instance with default values.
func newOptions() *options {
	defaultCopy := *http.DefaultClient

	return &options{
		tlsHandshakeTimeout:      20 * time.Second,
		skipHostnameVerification: false,
		middlewares: []TransportMiddleware{
			DialContextMiddleware(NewLocalDialBreaker(true)),
			FileProtocolMiddleware,
			ProxyMiddleware,
		},
		httpClient: &defaultCopy,
	}
}

// HTTPClient is the interface that is used to do http requests.
type HTTPClient interface {
	// Do sends an HTTP request and returns an HTTP response. The client will
	// follow policy (such as redirects, cookies, auth) as configured on the
	// client.
	Do(*http.Request) (*http.Response, error)
}

// Client represents an http client.
type Client struct {
	HTTPClient
}

// NewClient returns a new http client defined
// by the given config.
func NewClient(options ...Option) *Client {
	opts := newOptions()
	for _, option := range options {
		option(opts)
	}

	client := opts.httpClient
	transport := NewHTTPTLSTransport(TransportConfig{
		DisableKeepAlives:   opts.disableKeepAlives,
		TLSHandshakeTimeout: opts.tlsHandshakeTimeout,
		Middlewares:         opts.middlewares,
	})
	switch {
	case len(opts.caCertificates) > 0:
		transport = transportWithCerts(transport, opts.caCertificates, opts.skipHostnameVerification)
	case opts.skipHostnameVerification:
		transport = transportWithSkipVerify(transport, opts.skipHostnameVerification)
	}

	if opts.requestRecorder != nil {
		client.Transport = roundTripRecorder{
			requestRecorder:     opts.requestRecorder,
			wrappedRoundTripper: transport,
		}
	} else {
		client.Transport = transport
	}

	// Ensure we add the retry middleware after request recorder if there is
	// one, to ensure that we get all the logging at the right level.
	if opts.retryPolicy != nil {
		client.Transport = makeRetryMiddleware(
			client.Transport,
			*opts.retryPolicy,
			clock.WallClock,
		)
	}

	if opts.cookieJar != nil {
		client.Jar = opts.cookieJar
	}
	return &Client{HTTPClient: client}
}

func transportWithSkipVerify(defaultTransport *http.Transport, skipHostnameVerify bool) *http.Transport {
	transport := defaultTransport
	// We know that the DefaultHTTPTransport doesn't create a tls.Config here
	// so we can safely do that here.
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: skipHostnameVerify,
	}
	// We're creating a new tls.Config, HTTP/2 requests will not work, force the
	// client to create a HTTP/2 requests.
	transport.ForceAttemptHTTP2 = true
	return transport
}

func transportWithCerts(defaultTransport *http.Transport, caCerts []string, skipHostnameVerify bool) *http.Transport {
	pool := x509.NewCertPool()
	for _, cert := range caCerts {
		pool.AppendCertsFromPEM([]byte(cert))
	}

	tlsConfig := SecureTLSConfig()
	tlsConfig.RootCAs = pool
	tlsConfig.InsecureSkipVerify = skipHostnameVerify

	transport := defaultTransport
	transport.TLSClientConfig = tlsConfig

	// We're creating a new tls.Config, HTTP/2 requests will not work, force the
	// client to create a HTTP/2 requests.
	transport.ForceAttemptHTTP2 = true
	return transport
}

// Get issues a GET to the specified URL.  It mimics the net/http Get,
// but allows for enhanced debugging.
//
// When err is nil, resp always contains a non-nil resp.Body.
// Caller should close resp.Body when done reading from it.
func (c *Client) Get(ctx context.Context, path string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req)
}
