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
	"strings"
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

// RemoteBackend posts each scanned page to the remote odi server's
// /api/v1/upload endpoint as soon as ProcessPage is called.
type RemoteBackend struct {
	baseURL string
	token   string
	client  *http.Client
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
	return &RemoteBackend{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		client:  client,
	}, nil
}

func (r *RemoteBackend) ProcessPage(ctx context.Context, page models.ScannedPage) error {
	data, err := io.ReadAll(page.Reader)
	if err != nil {
		return fmt.Errorf("remote backend: read page seq=%d: %w", page.SequenceID, err)
	}

	ur, err := r.uploadPage(ctx, remotePage{scanID: page.ScanID, sequenceID: page.SequenceID, data: data})
	if err != nil {
		return err
	}
	if ur.Failed > 0 {
		return fmt.Errorf("remote backend: server reported %d failed pages (scan %s)", ur.Failed, ur.ScanID)
	}
	log.Infof("remote upload: scan=%s seq=%d processed=%d duplicates=%d", ur.ScanID, page.SequenceID, ur.Processed, ur.Duplicates)
	return nil
}

// Flush is a no-op for RemoteBackend because pages are uploaded immediately.
func (r *RemoteBackend) Flush(context.Context) error { return nil }

func (r *RemoteBackend) uploadPage(ctx context.Context, page remotePage) (UploadResponse, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if page.scanID != "" {
		if err := w.WriteField("scanID", page.scanID); err != nil {
			return UploadResponse{}, fmt.Errorf("remote backend: multipart scanID: %w", err)
		}
	}
	if err := w.WriteField("sequenceOffset", fmt.Sprintf("%d", page.sequenceID-1)); err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: multipart sequenceOffset: %w", err)
	}
	fw, err := w.CreateFormFile("files", fmt.Sprintf("page-%04d.jpg", page.sequenceID))
	if err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: multipart create: %w", err)
	}
	if _, err := fw.Write(page.data); err != nil {
		return UploadResponse{}, fmt.Errorf("remote backend: multipart write: %w", err)
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
		return UploadResponse{}, fmt.Errorf("remote backend: upload page seq=%d: %w", page.sequenceID, err)
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
