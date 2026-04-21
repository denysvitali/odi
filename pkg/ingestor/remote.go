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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/denysvitali/odi/pkg/models"
)

// RemoteBackendConfig configures a RemoteBackend.
type RemoteBackendConfig struct {
	// BaseURL is the odi server root, e.g. "https://odi.example.com".
	BaseURL string
	// Token, when set, is sent as a Bearer Authorization header.
	Token string
	// HTTPClient is optional. A default client with a 2 minute timeout is used otherwise.
	HTTPClient *http.Client
}

// RemoteBackend buffers scanned pages in memory and flushes them in one
// or more multipart POSTs to the remote odi server's /api/v1/upload endpoint.
type RemoteBackend struct {
	baseURL string
	token   string
	client  *http.Client

	mu    sync.Mutex
	pages []remotePage
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

const remoteUploadChunkSize = 25

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
	return &RemoteBackend{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		client:  client,
	}, nil
}

func (r *RemoteBackend) ProcessPage(_ context.Context, page models.ScannedPage) error {
	data, err := io.ReadAll(page.Reader)
	if err != nil {
		return fmt.Errorf("remote backend: read page seq=%d: %w", page.SequenceID, err)
	}
	r.mu.Lock()
	r.pages = append(r.pages, remotePage{scanID: page.ScanID, sequenceID: page.SequenceID, data: data})
	r.mu.Unlock()
	return nil
}

// Flush uploads all buffered pages in bounded multipart requests.
func (r *RemoteBackend) Flush(ctx context.Context) error {
	r.mu.Lock()
	pages := r.pages
	r.pages = nil
	r.mu.Unlock()

	if len(pages) == 0 {
		return nil
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].sequenceID < pages[j].sequenceID
	})

	scanID := pages[0].scanID
	totalProcessed := 0
	totalDuplicates := 0
	for start := 0; start < len(pages); start += remoteUploadChunkSize {
		end := start + remoteUploadChunkSize
		if end > len(pages) {
			end = len(pages)
		}
		ur, err := r.uploadChunk(ctx, scanID, pages[start:end])
		if err != nil {
			return err
		}
		if scanID == "" {
			scanID = ur.ScanID
		}
		totalProcessed += ur.Processed
		totalDuplicates += ur.Duplicates
		if ur.Failed > 0 {
			return fmt.Errorf("remote backend: server reported %d failed pages (scan %s)", ur.Failed, ur.ScanID)
		}
	}
	log.Infof("remote upload: scan=%s processed=%d duplicates=%d", scanID, totalProcessed, totalDuplicates)
	return nil
}

func (r *RemoteBackend) uploadChunk(ctx context.Context, scanID string, pages []remotePage) (UploadResponse, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if scanID != "" {
		if err := w.WriteField("scanID", scanID); err != nil {
			return UploadResponse{}, fmt.Errorf("remote backend: multipart scanID: %w", err)
		}
	}
	if len(pages) > 0 {
		if err := w.WriteField("sequenceOffset", fmt.Sprintf("%d", pages[0].sequenceID-1)); err != nil {
			return UploadResponse{}, fmt.Errorf("remote backend: multipart sequenceOffset: %w", err)
		}
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
		return UploadResponse{}, fmt.Errorf("remote backend: upload %d pages: %w", len(pages), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return UploadResponse{}, fmt.Errorf("remote backend: upload failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(buf)))
	}

	var ur UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: decode upload response: %w", err)
	}
	log.Infof("remote upload chunk: scan=%s pages=%d processed=%d duplicates=%d failed=%d", ur.ScanID, len(pages), ur.Processed, ur.Duplicates, ur.Failed)
	return ur, nil
}

func (r *RemoteBackend) Close() error { return nil }

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
		return fmt.Errorf("remote backend: ping: %w", err)
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
				failing = append(failing, fmt.Sprintf("%s: %s", ch.Name, ch.Detail))
			}
		}
		if len(failing) > 0 {
			return fmt.Errorf("remote backend not ready (HTTP %d): %s", resp.StatusCode, strings.Join(failing, "; "))
		}
	}
	return fmt.Errorf("remote backend not ready: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
