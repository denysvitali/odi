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
	"time"
)

const (
	DefaultMaxConcurrency = 5
	DefaultMaxRetries     = 3
	DefaultInitialBackoff = 500 * time.Millisecond
	DefaultMaxBackoff     = 10 * time.Second
	DefaultTimeout        = 60 * time.Second
)

type Client struct {
	http     *http.Client
	endpoint *url.URL

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

	ocrUrl, err := c.endpoint.Parse("/api/v1/ocr")
	if err != nil {
		return nil, fmt.Errorf("unable to parse URL: %w", err)
	}

	backoff := c.initialBackoff
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, retryable, err := c.doOnce(ctx, ocrUrl.String(), data)
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

func (c *Client) doOnce(ctx context.Context, ocrUrl string, data []byte) (*OcrResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrUrl, bytes.NewReader(data))
	if err != nil {
		return nil, false, fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("Content-Type", "image/jpeg")
	req.ContentLength = int64(len(data))

	res, err := c.http.Do(req)
	if err != nil {
		return nil, isRetryableErr(err), fmt.Errorf("unable to perform HTTP request: %w", err)
	}
	defer res.Body.Close()

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
	healthEndpoint, err := c.endpoint.Parse("/healthz")
	if err != nil {
		return false, err
	}
	res, err := c.http.Get(healthEndpoint.String())
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

func New(endpoint string, opts ...Option) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("scheme %s is not supported", u.Scheme)
	}

	c := &Client{
		endpoint: u,
		http: &http.Client{
			Timeout: DefaultTimeout,
		},
		sem:            make(chan struct{}, DefaultMaxConcurrency),
		maxRetries:     DefaultMaxRetries,
		initialBackoff: DefaultInitialBackoff,
		maxBackoff:     DefaultMaxBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) SetHTTPTransport(transport http.RoundTripper) {
	c.http.Transport = transport
}
