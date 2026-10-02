package ocrclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	DefaultMaxConcurrency = 5
	DefaultMaxRetries     = 3
	DefaultInitialBackoff = 500 * time.Millisecond
	DefaultMaxBackoff     = 10 * time.Second
	DefaultTimeout        = 60 * time.Second

	// HeaderIdempotencyKey is the request header attached to OCR requests so
	// the server can deduplicate retried submissions of the same logical
	// request. The same key is reused across every retry.
	HeaderIdempotencyKey = "Idempotency-Key"

	// EnvAllowPrivateTargets, when set to "true", disables the SSRF guard that
	// rejects loopback/link-local/private IP destinations. Intended for local
	// development against on-host OCR servers.
	EnvAllowPrivateTargets = "OCR_ALLOW_PRIVATE_TARGETS"
	// EnvAllowedHosts optionally restricts explicit private-target consent to configured hosts.
	EnvAllowedHosts = "OCR_ALLOWED_HOSTS"
)

var log = logrus.StandardLogger().WithField("package", "ocrclient")

type Client struct {
	http                *http.Client
	endpoint            *url.URL
	allowPrivateTargets bool

	sem            chan struct{}
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

type Option func(*Client)

func WithMaxConcurrency(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.sem = make(chan struct{}, n)
		}
	}
}

func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

func WithInitialBackoff(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.initialBackoff = d
		}
	}
}

func WithMaxBackoff(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.maxBackoff = d
		}
	}
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.http.Timeout = d
		}
	}
}

// Process submits the image for OCR, respecting the configured concurrency
// limit and retrying transient failures with exponential backoff + jitter.
// A single Idempotency-Key is generated once per logical request and reused
// across every retry attempt so the OCR server can safely deduplicate.
func (c *Client) Process(ctx context.Context, f io.Reader) (*OcrResult, error) {
	// Buffer the payload so we can retry.
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("unable to read input: %w", err)
	}

	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()

	ocrURL, err := c.endpoint.Parse("/api/v1/ocr")
	if err != nil {
		return nil, fmt.Errorf("unable to parse URL: %w", err)
	}

	// Generate ONE idempotency key per logical request. Reusing it across
	// retries lets the OCR server collapse duplicate submissions caused by
	// transient client-side retries.
	idemKey := uuid.NewString()

	backoff := c.initialBackoff
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, retryable, err := c.doOnce(ctx, ocrURL.String(), idemKey, data)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retryable || attempt == c.maxRetries {
			break
		}

		sleep := jitter(backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(sleep):
		}
		backoff *= 2
		if backoff > c.maxBackoff {
			backoff = c.maxBackoff
		}
	}
	return nil, lastErr
}

func (c *Client) doOnce(ctx context.Context, ocrURL string, idempotencyKey string, data []byte) (*OcrResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrURL, bytes.NewReader(data))
	if err != nil {
		return nil, false, fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("Content-Type", "image/jpeg")
	if idempotencyKey != "" {
		req.Header.Set(HeaderIdempotencyKey, idempotencyKey)
	}
	req.ContentLength = int64(len(data))

	res, err := c.http.Do(req)
	if err != nil {
		return nil, isRetryableErr(err), fmt.Errorf("unable to perform HTTP request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		retry := res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500
		return nil, retry, fmt.Errorf("unexpected status %s", res.Status)
	}

	var ocrResult OcrResult
	if err := json.NewDecoder(res.Body).Decode(&ocrResult); err != nil {
		return nil, false, err
	}
	return &ocrResult, false, nil
}

func isRetryableErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Caller's context being cancelled is not retryable; the per-request
		// timeout from http.Client.Timeout surfaces as url.Error with a
		// net.Error Timeout() == true, handled below.
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	// Treat generic network errors (connection refused, reset, EOF mid-body) as retryable.
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	// full jitter: [d/2, d + d/2)
	half := d / 2
	//nolint:gosec // math/rand is sufficient for jitter; not security-sensitive.
	return half + time.Duration(rand.Int63n(int64(d)))
}

func (c *Client) acquire(ctx context.Context) error {
	if c.sem == nil {
		return nil
	}
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) release() {
	if c.sem == nil {
		return
	}
	<-c.sem
}

// Healthz checks if the OCR service is healthy and returns true if it is.
func (c *Client) Healthz() (bool, error) {
	return c.HealthzContext(context.Background())
}

// HealthzContext checks OCR availability using the caller's cancellation deadline.
func (c *Client) HealthzContext(ctx context.Context) (bool, error) {
	healthEndpoint, err := c.endpoint.Parse("/healthz")
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthEndpoint.String(), nil)
	if err != nil {
		return false, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

// validateOCRTarget rejects URLs that point at loopback, link-local, or RFC
// 1918 private ranges (including the AWS instance metadata service at
// 169.254.169.254). When the OCR_ALLOW_PRIVATE_TARGETS env var is set to
// "true" the guard is bypassed but a warning is emitted.
func validateOCRTargetContext(ctx context.Context, u *url.URL) error {
	if u == nil {
		return errors.New("nil URL")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("OCR target has no host")
	}

	allow := strings.EqualFold(ocrEnv(EnvAllowPrivateTargets), "true")
	if allow {
		if hosts := strings.TrimSpace(ocrEnv(EnvAllowedHosts)); hosts != "" {
			matched := false
			for _, allowedHost := range strings.Split(hosts, ",") {
				if strings.EqualFold(strings.TrimSpace(allowedHost), host) {
					matched = true
					break
				}
			}
			if !matched {
				return errors.New("OCR target is outside the configured host allowlist")
			}
		}
		log.Warnf("%s=true: OCR client SSRF guard disabled; private/loopback OCR targets are permitted", EnvAllowPrivateTargets)
		return nil
	}

	// Fast path: literal hostnames that always map to local/metadata services.
	lower := strings.ToLower(host)
	switch lower {
	case "localhost", "ip6-localhost", "ip6-loopback":
		return fmt.Errorf("OCR target %q resolves to loopback; set %s=true to allow", host, EnvAllowPrivateTargets)
	case "metadata.google.internal":
		return fmt.Errorf("OCR target %q is a cloud metadata endpoint; set %s=true to allow", host, EnvAllowPrivateTargets)
	}

	// If host is an IP literal, validate it directly.
	if ip := net.ParseIP(host); ip != nil {
		if err := checkIP(ip); err != nil {
			return fmt.Errorf("OCR target %q: %w (set %s=true to allow)", host, err, EnvAllowPrivateTargets)
		}
		return nil
	}

	// Hostname → IP resolution. Reject if ANY resolved address is private.
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		// Treat resolution failures as fatal so we don't accidentally bypass
		// the guard when DNS is broken at startup.
		return fmt.Errorf("unable to resolve OCR target %q: %w", host, err)
	}
	for _, ip := range ips {
		if err := checkIP(ip.IP); err != nil {
			return fmt.Errorf("OCR target %q resolves to %s: %w (set %s=true to allow)", host, ip.IP, err, EnvAllowPrivateTargets)
		}
	}
	return nil
}

// checkIP rejects loopback, unspecified, link-local, multicast, and RFC
// 1918/4193 private addresses, as well as the AWS instance metadata IP.
func checkIP(ip net.IP) error {
	if ip.IsLoopback() {
		return errors.New("loopback address not permitted")
	}
	if ip.IsUnspecified() {
		return errors.New("unspecified address not permitted")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return errors.New("link-local address not permitted")
	}
	if ip.IsMulticast() {
		return errors.New("multicast address not permitted")
	}
	if ip.IsPrivate() {
		return errors.New("private address not permitted")
	}
	// AWS / OpenStack instance-metadata: 169.254.169.254 — already caught by
	// IsLinkLocalUnicast, but keep an explicit check for clarity.
	if ip.Equal(net.IPv4(169, 254, 169, 254)) {
		return errors.New("cloud metadata address not permitted")
	}
	return nil
}

func New(endpoint string, opts ...Option) (*Client, error) {
	return NewWithContext(context.Background(), endpoint, opts...)
}

// NewWithContext also bounds startup DNS resolution by the caller's context.
func NewWithContext(ctx context.Context, endpoint string, opts ...Option) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("scheme %s is not supported", u.Scheme)
	}

	if u.User != nil {
		return nil, errors.New("OCR target must not contain credentials")
	}
	if err := validateOCRTargetContext(ctx, u); err != nil {
		return nil, err
	}

	c := &Client{
		endpoint:            u,
		allowPrivateTargets: strings.EqualFold(ocrEnv(EnvAllowPrivateTargets), "true"),
		http: &http.Client{
			Timeout:       DefaultTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		sem:            make(chan struct{}, DefaultMaxConcurrency),
		maxRetries:     DefaultMaxRetries,
		initialBackoff: DefaultInitialBackoff,
		maxBackoff:     DefaultMaxBackoff,
	}
	if err := c.SetHTTPTransport(http.DefaultTransport.(*http.Transport)); err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// SetHTTPTransport preserves custom TLS roots while enforcing destination policy.
// Opaque RoundTrippers cannot guarantee which address receives a document.
func (c *Client) SetHTTPTransport(transport http.RoundTripper) error {
	var base *http.Transport
	switch t := transport.(type) {
	case *http.Transport:
		base = t
	case interface{ HTTPTransport() *http.Transport }:
		base = t.HTTPTransport()
	default:
		return errors.New("OCR transport must expose an HTTP transport for destination validation")
	}
	if base == nil {
		return errors.New("nil OCR HTTP transport")
	}
	t := base.Clone()
	// Proxies and custom TLS dialers would bypass the validated direct dial.
	t.Proxy = nil
	t.DialTLS = nil //nolint:staticcheck // Clear deprecated TLS dial hooks so they cannot bypass the guarded dialer.
	t.DialTLSContext = nil
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = guardedDialContext(c.allowPrivateTargets, net.DefaultResolver.LookupIPAddr, dialer.DialContext)
	c.http.Transport = t
	return nil
}

func guardedDialContext(allowPrivate bool,
	resolve func(context.Context, string) ([]net.IPAddr, error),
	dial func(context.Context, string, string) (net.Conn, error),
) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid OCR destination address")
		}
		ips, err := resolve(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve OCR destination: %w", err)
		}
		if len(ips) == 0 {
			return nil, errors.New("OCR destination resolved to no addresses")
		}
		// Check every answer before dialing any, then dial the checked numeric IP.
		for _, addr := range ips {
			if !allowPrivate {
				if err := checkIP(addr.IP); err != nil {
					return nil, fmt.Errorf("OCR destination rejected: %w", err)
				}
			}
		}
		var lastErr error
		for _, addr := range ips {
			ip := addr.IP.String()
			if addr.Zone != "" {
				ip += "%" + addr.Zone
			}
			conn, err := dial(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, lastErr
	}
}

// Prefer ODI-prefixed configuration while preserving the legacy variable names.
func ocrEnv(name string) string {
	if value, present := os.LookupEnv("ODI_" + name); present {
		return value
	}
	return os.Getenv(name)
}
