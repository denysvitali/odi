package ingestor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/odi/pkg/ingestor"
	"github.com/denysvitali/odi/pkg/models"
)

func TestRemoteBackend_FlushPostsAllBufferedPages(t *testing.T) {
	var gotAuth string
	var gotFiles []string
	var gotContentType string
	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/upload", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		requestCount.Add(1)
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		require.NoError(t, r.ParseMultipartForm(32<<20))
		for _, fh := range r.MultipartForm.File["files"] {
			gotFiles = append(gotFiles, fh.Filename)
			f, err := fh.Open()
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, f)
			f.Close()
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"scanID":    "abc-123",
			"processed": len(gotFiles),
			"failed":    0,
		})
	}))
	defer srv.Close()

	b, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{
		BaseURL: srv.URL,
		Token:   "secret-token",
	})
	require.NoError(t, err)

	ctx := context.Background()
	for seq := 1; seq <= 3; seq++ {
		page := models.ScannedPage{
			Reader:     bytes.NewReader([]byte("jpeg-bytes-for-page")),
			ScanID:     "ignored-client-id",
			SequenceID: seq,
		}
		require.NoError(t, b.ProcessPage(ctx, page))
	}

	// No request should have been sent yet — ProcessPage only buffers.
	require.Zero(t, requestCount.Load())

	require.NoError(t, b.Flush(ctx))
	require.Equal(t, int32(1), requestCount.Load())
	assert.Equal(t, "Bearer secret-token", gotAuth)
	assert.True(t, strings.HasPrefix(gotContentType, "multipart/form-data"), "got %q", gotContentType)
	assert.Len(t, gotFiles, 3)

	// Flushing again with no buffered pages must be a no-op.
	require.NoError(t, b.Flush(ctx))
	require.Equal(t, int32(1), requestCount.Load())
}

func TestRemoteBackend_FlushChunksLargeUploads(t *testing.T) {
	var requestCount atomic.Int32
	var sequenceOffsets []string
	var scanIDs []string
	var filesPerRequest []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/upload", r.URL.Path)
		requestCount.Add(1)
		require.NoError(t, r.ParseMultipartForm(32<<20))

		sequenceOffsets = append(sequenceOffsets, r.MultipartForm.Value["sequenceOffset"][0])
		scanIDs = append(scanIDs, r.MultipartForm.Value["scanID"][0])
		files := r.MultipartForm.File["files"]
		filesPerRequest = append(filesPerRequest, len(files))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"scanID":     r.MultipartForm.Value["scanID"][0],
			"processed":  len(files),
			"duplicates": 0,
			"failed":     0,
		})
	}))
	defer srv.Close()

	b, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: srv.URL})
	require.NoError(t, err)

	ctx := context.Background()
	for seq := 1; seq <= 60; seq++ {
		require.NoError(t, b.ProcessPage(ctx, models.ScannedPage{
			Reader:     bytes.NewReader([]byte("jpeg-bytes-for-page")),
			ScanID:     "89aefd17-4e1c-4339-bbc7-3bd0ca40a34c",
			SequenceID: seq,
		}))
	}

	require.NoError(t, b.Flush(ctx))
	require.Equal(t, int32(3), requestCount.Load())
	assert.Equal(t, []string{"0", "25", "50"}, sequenceOffsets)
	assert.Equal(t, []int{25, 25, 10}, filesPerRequest)
	assert.Equal(t, []string{
		"89aefd17-4e1c-4339-bbc7-3bd0ca40a34c",
		"89aefd17-4e1c-4339-bbc7-3bd0ca40a34c",
		"89aefd17-4e1c-4339-bbc7-3bd0ca40a34c",
	}, scanIDs)
}

func TestRemoteBackend_PingReady(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/readyz", r.URL.Path)
		hit = true
		_ = json.NewEncoder(w).Encode(ingestor.ReadinessResponse{
			Ready: true,
			Checks: []ingestor.ReadinessCheck{
				{Name: "opensearch", OK: true},
				{Name: "indexer", OK: true},
				{Name: "ocr", OK: true},
				{Name: "zefix", OK: true},
			},
		})
	}))
	defer srv.Close()

	b, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: srv.URL})
	require.NoError(t, err)
	require.NoError(t, b.Ping(context.Background()))
	require.True(t, hit)
}

func TestRemoteBackend_PingNotReadyReportsFailedChecks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ingestor.ReadinessResponse{
			Ready: false,
			Checks: []ingestor.ReadinessCheck{
				{Name: "opensearch", OK: true},
				{Name: "indexer", OK: false, Detail: "indexer not configured"},
				{Name: "ocr", OK: false, Detail: "connection refused"},
			},
		})
	}))
	defer srv.Close()

	b, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: srv.URL})
	require.NoError(t, err)
	err = b.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "indexer: indexer not configured")
	assert.Contains(t, err.Error(), "ocr: connection refused")
}

func TestRemoteBackend_FlushServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	b, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: srv.URL})
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, b.ProcessPage(ctx, models.ScannedPage{
		Reader: bytes.NewReader([]byte("x")), SequenceID: 1,
	}))
	err = b.Flush(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestRemoteBackend_RejectsBadURL(t *testing.T) {
	_, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: ""})
	assert.Error(t, err)

	_, err = ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: "ftp://nope"})
	assert.Error(t, err)
}

func TestIngestor_ScanPagesWithRemoteBackend(t *testing.T) {
	// End-to-end wiring: scanner -> Ingestor -> RemoteBackend -> fake server.
	var processed int
	var scanIDs = map[string]struct{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(32<<20))
		files := r.MultipartForm.File["files"]
		processed += len(files)
		for _, fh := range files {
			scanIDs[fh.Filename] = struct{}{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"scanID": "server-scan", "processed": len(files), "failed": 0,
		})
	}))
	defer srv.Close()

	b, err := ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{BaseURL: srv.URL})
	require.NoError(t, err)

	ing := ingestor.NewWithBackend(b)
	scanner := &testScanner{files: []io.Reader{
		bytes.NewReader([]byte("page-1")),
		bytes.NewReader([]byte("page-2")),
	}}
	require.NoError(t, ing.ScanPages(context.Background(), scanner, 2))
	assert.Equal(t, 2, processed)
}
