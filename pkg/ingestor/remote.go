package ingestor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/denysvitali/odi/pkg/models"
)

// Batching tunables for RemoteBackend. These bound how aggressively the
// backend buffers pages before flushing them in one multipart upload.
const (
	// DefaultRemoteBatchMaxPages flushes once this many pages are buffered.
	DefaultRemoteBatchMaxPages = 10
	// DefaultRemoteBatchMaxBytes flushes when the buffered payload exceeds this many bytes.
	DefaultRemoteBatchMaxBytes = 50 * 1024 * 1024 // 50 MB
	// DefaultRemoteBatchIdleFlush flushes when no new page has arrived for this long.
	DefaultRemoteBatchIdleFlush = 2 * time.Second
)

// RemoteBackendConfig configures a RemoteBackend.
type RemoteBackendConfig struct {
	// BaseURL is the odi server root, e.g. "https://odi.example.com".
	BaseURL string
	// Token, when set, is sent as a Bearer Authorization header.
	Token string
	// HTTPClient is optional. A default client with a 2 minute timeout is used otherwise.
	HTTPClient *http.Client

	// BatchMaxPages overrides DefaultRemoteBatchMaxPages when > 0.
	BatchMaxPages int
	// BatchMaxBytes overrides DefaultRemoteBatchMaxBytes when > 0.
	BatchMaxBytes int
	// BatchIdleFlush overrides DefaultRemoteBatchIdleFlush when > 0.
	BatchIdleFlush time.Duration
}

// RemoteBackend buffers scanned pages in memory and posts them to the remote
// odi server's /api/v1/upload endpoint in batches. Batches flush when any of
// the following triggers fires:
//
//   - the buffer holds BatchMaxPages pages, or
//   - the buffer total payload exceeds BatchMaxBytes, or
//   - no page has arrived in BatchIdleFlush.
//
// Flush() forces a synchronous final flush of any remaining pages.
type RemoteBackend struct {
	baseURL string
	token   string
	client  *http.Client

	maxPages  int
	maxBytes  int
	idleFlush time.Duration

	mu        sync.Mutex
	buf       []remotePage
	bufBytes  int
	currScan  string
	flushErr  error
	closed    bool
	idleTimer *time.Timer

	flushMu sync.Mutex // serializes actual HTTP flushes
}

type remotePage struct {
	scanID     string
	sequenceID int
	data       []byte
}

// UploadResponse mirrors the server's upload response body (subset).
type UploadResponse struct {
	ScanID     string `json:"scanID"`
	Processed  int    `json:"processed"`
	Duplicates int    `json:"duplicates"`
	Failed     int    `json:"failed"`
}

func NewRemoteBackend(cfg RemoteBackendConfig) (*RemoteBackend, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("remote backend: BaseURL is required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("remote backend: invalid BaseURL %q: %w", cfg.BaseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("remote backend: BaseURL must be http(s), got %q", u.Scheme)
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}

	maxPages := cfg.BatchMaxPages
	if maxPages <= 0 {
		maxPages = DefaultRemoteBatchMaxPages
	}
	maxBytes := cfg.BatchMaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultRemoteBatchMaxBytes
	}
	idle := cfg.BatchIdleFlush
	if idle <= 0 {
		idle = DefaultRemoteBatchIdleFlush
	}

	return &RemoteBackend{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		token:     cfg.Token,
		client:    client,
		maxPages:  maxPages,
		maxBytes:  maxBytes,
		idleFlush: idle,
	}, nil
}

// ProcessPage buffers a page for later batched upload. If a previous async
// flush failed it returns that error so callers know to stop the scan.
func (r *RemoteBackend) ProcessPage(ctx context.Context, page models.ScannedPage) error {
	data, err := io.ReadAll(page.Reader)
	if err != nil {
		return fmt.Errorf("remote backend: read page seq=%d: %w", page.SequenceID, err)
	}

	r.mu.Lock()
	if err := r.flushErr; err != nil {
		r.mu.Unlock()
		return fmt.Errorf("remote backend: previous async flush failed: %w", err)
	}
	if r.closed {
		r.mu.Unlock()
		return fmt.Errorf("remote backend: closed")
	}

	// If the scan id changes mid-stream, flush the current batch first so we
	// never mix two scans in one upload.
	if r.currScan != "" && page.ScanID != "" && r.currScan != page.ScanID {
		toFlush := r.takeBufferLocked()
		r.mu.Unlock()
		if err := r.flushBatch(ctx, toFlush); err != nil {
			return err
		}
		r.mu.Lock()
	}
	if page.ScanID != "" {
		r.currScan = page.ScanID
	}

	r.buf = append(r.buf, remotePage{
		scanID:     page.ScanID,
		sequenceID: page.SequenceID,
		data:       data,
	})
	r.bufBytes += len(data)
	r.resetIdleTimerLocked(ctx)

	if len(r.buf) >= r.maxPages || r.bufBytes >= r.maxBytes {
		toFlush := r.takeBufferLocked()
		r.mu.Unlock()
		return r.flushBatch(ctx, toFlush)
	}
	r.mu.Unlock()
	return nil
}

// Flush forces a synchronous flush of any buffered pages.
func (r *RemoteBackend) Flush(ctx context.Context) error {
	r.mu.Lock()
	if r.idleTimer != nil {
		r.idleTimer.Stop()
		r.idleTimer = nil
	}
	if err := r.flushErr; err != nil {
		r.flushErr = nil
		r.mu.Unlock()
		return fmt.Errorf("remote backend: previous async flush failed: %w", err)
	}
	toFlush := r.takeBufferLocked()
	r.mu.Unlock()
	if len(toFlush) == 0 {
		return nil
	}
	return r.flushBatch(ctx, toFlush)
}

// Close stops the idle timer and refuses further pages.
func (r *RemoteBackend) Close() error {
	r.mu.Lock()
	r.closed = true
	if r.idleTimer != nil {
		r.idleTimer.Stop()
		r.idleTimer = nil
	}
	r.mu.Unlock()
	return nil
}

// resetIdleTimerLocked must be called with r.mu held.
func (r *RemoteBackend) resetIdleTimerLocked(ctx context.Context) {
	if r.idleTimer != nil {
		r.idleTimer.Stop()
	}
	r.idleTimer = time.AfterFunc(r.idleFlush, func() {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return
		}
		toFlush := r.takeBufferLocked()
		r.mu.Unlock()
		if len(toFlush) == 0 {
			return
		}
		if err := r.flushBatch(ctx, toFlush); err != nil {
			r.mu.Lock()
			if r.flushErr == nil {
				r.flushErr = err
			}
			r.mu.Unlock()
			log.Warnf("remote backend: idle flush failed: %v", sanitizeForLogErr(err))
		}
	})
}

// takeBufferLocked drains the page buffer. Must be called with r.mu held.
func (r *RemoteBackend) takeBufferLocked() []remotePage {
	if len(r.buf) == 0 {
		return nil
	}
	out := r.buf
	r.buf = nil
	r.bufBytes = 0
	return out
}

// flushBatch posts a batch of pages in a single multipart upload.
func (r *RemoteBackend) flushBatch(ctx context.Context, pages []remotePage) error {
	if len(pages) == 0 {
		return nil
	}
	// Serialize actual HTTP flushes so an idle-timer flush and a caller-driven
	// flush cannot race each other.
	r.flushMu.Lock()
	defer r.flushMu.Unlock()

	ur, err := r.uploadBatch(ctx, pages)
	if err != nil {
		return err
	}
	if ur.Failed > 0 {
		return fmt.Errorf("remote backend: server reported %d failed pages (scan %s)", ur.Failed, ur.ScanID)
	}
	first := pages[0].sequenceID
	last := pages[len(pages)-1].sequenceID
	log.Infof("remote upload: scan=%s seq=%d..%d count=%d processed=%d duplicates=%d", ur.ScanID, first, last, len(pages), ur.Processed, ur.Duplicates)
	return nil
}

func (r *RemoteBackend) uploadBatch(ctx context.Context, pages []remotePage) (UploadResponse, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	scanID := pages[0].scanID
	if scanID != "" {
		if err := w.WriteField("scanID", scanID); err != nil {
			return UploadResponse{}, fmt.Errorf("remote backend: multipart scanID: %w", err)
		}
	}
	if err := w.WriteField("sequenceOffset", fmt.Sprintf("%d", pages[0].sequenceID-1)); err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: multipart sequenceOffset: %w", err)
	}
	for _, p := range pages {
		fw, err := w.CreateFormFile("files", fmt.Sprintf("page-%04d.jpg", p.sequenceID))
		if err != nil {
			return UploadResponse{}, fmt.Errorf("remote backend: multipart create: %w", err)
		}
		if _, err := fw.Write(p.data); err != nil {
			return UploadResponse{}, fmt.Errorf("remote backend: multipart write: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: multipart close: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/api/v1/upload", body)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: upload pages count=%d firstSeq=%d: %w", len(pages), pages[0].sequenceID, sanitizeForLogErr(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return UploadResponse{}, fmt.Errorf("remote backend: upload failed: HTTP %d: %s", resp.StatusCode, sanitizeForLog(strings.TrimSpace(string(buf))))
	}

	var ur UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: decode upload response: %w", err)
	}
	return ur, nil
}

// ReadinessCheck is one line of the server's /readyz response.
type ReadinessCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// ReadinessResponse is the full /readyz payload.
type ReadinessResponse struct {
	Ready  bool             `json:"ready"`
	Checks []ReadinessCheck `json:"checks"`
}

// Ping verifies the remote server is ready to accept ingestion by calling
// /readyz. A non-ready server (503) yields an error that names every failed
// dependency so the operator knows what to fix.
func (r *RemoteBackend) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/readyz", nil)
	if err != nil {
		return fmt.Errorf("remote backend: ping build request: %w", err)
	}
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("remote backend: ping: %w", sanitizeForLogErr(err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	var ready ReadinessResponse
	// The body may be empty or non-JSON on unexpected status codes.
	_ = json.Unmarshal(body, &ready)

	if resp.StatusCode == http.StatusOK && ready.Ready {
		return nil
	}

	if len(ready.Checks) > 0 {
		var failing []string
		for _, ch := range ready.Checks {
			if !ch.OK {
				failing = append(failing, fmt.Sprintf("%s: %s", ch.Name, sanitizeForLog(ch.Detail)))
			}
		}
		if len(failing) > 0 {
			return fmt.Errorf("remote backend not ready (HTTP %d): %s", resp.StatusCode, strings.Join(failing, "; "))
		}
	}
	return fmt.Errorf("remote backend not ready: HTTP %d: %s", resp.StatusCode, sanitizeForLog(strings.TrimSpace(string(body))))
}

// sanitizeForLog redacts anything that looks like a bearer token or an
// Authorization header from a free-form string before it ends up in a log
// line or wrapped error.
var (
	bearerTokenRe = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-_\.=:+/]+`)
	authHeaderRe  = regexp.MustCompile(`(?i)authorization\s*:\s*[^\r\n]+`)
)

func sanitizeForLog(s string) string {
	if s == "" {
		return s
	}
	s = authHeaderRe.ReplaceAllString(s, "Authorization: [REDACTED]")
	s = bearerTokenRe.ReplaceAllString(s, "Bearer [REDACTED]")
	return s
}

// sanitizeForLogErr returns an error whose message has been run through
// sanitizeForLog. Useful when wrapping errors from net/http whose textual
// form may contain the request URL including credentials in some edge cases.
func sanitizeForLogErr(err error) error {
	if err == nil {
		return nil
	}
	clean := sanitizeForLog(err.Error())
	if clean == err.Error() {
		return err
	}
	return fmt.Errorf("%s", clean)
}
