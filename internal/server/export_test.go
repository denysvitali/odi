package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	odicrypt "github.com/denysvitali/odi/pkg/crypt"
)

// exportTestServer wires handleExport to a fake OpenSearch backend and an
// in-memory RW storage. The osHandler receives the raw OpenSearch requests so
// tests can both assert on the query body and drive the scroll lifecycle.
func exportTestServer(t *testing.T, apiToken string, store *mockRWStorage, osHandler http.HandlerFunc) *mockServer {
	t.Helper()

	osSrv := httptest.NewServer(osHandler)
	t.Cleanup(osSrv.Close)

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{
		e:        router,
		osClient: osClient,
		osIndex:  "documents",
		storage:  store,
		apiToken: apiToken,
	}
	router.POST("/api/v1/export", s.handleExport)

	return &mockServer{Server: s, router: router}
}

// TestExport_FilterWiring asserts that the export query reuses
// buildSearchFilters: a company filter must appear in the OpenSearch query body.
func TestExport_FilterWiring(t *testing.T) {
	var capturedSearchBody map[string]any

	store := newMockRWStorage()
	store.addPage("scan-a", 0, []byte("PAGE-A-CONTENTS"))

	ms := exportTestServer(t, "test-secret", store, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/documents/_search":
			capturedSearchBody = decodeSearchBody(t, r)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits": map[string]any{
					"hits": []map[string]any{
						{"_id": "scan-a_0", "_source": map[string]any{
							"scanID": "scan-a", "sequenceID": 0, "contentDigest": "deadbeef",
						}},
					},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			// Exhaust the scroll on the first continuation.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": []map[string]any{}},
			})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	})

	payload := `{"companies":["Swisscom"],"passphrase":"correct horse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// The query must wrap a bool/filter clause referencing the company name.
	query, ok := capturedSearchBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")
	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "company filter must produce a bool query")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok)
	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "Swisscom")
	assert.Contains(t, string(filterJSON), "company.name.keyword")
}

// TestExport_DecryptAndVerifyManifest exercises the full happy path: the
// response body must decrypt with the request passphrase via pkg/crypt, the
// archive must contain the page file plus a manifest, and the manifest
// signature must verify under the server secret.
func TestExport_DecryptAndVerifyManifest(t *testing.T) {
	const secret = "server-api-token"
	const passphrase = "open-sesame"
	const pageContents = "THE-ACTUAL-SCANNED-BYTES"

	store := newMockRWStorage()
	store.addPage("scan-x", 2, []byte(pageContents))

	ms := exportTestServer(t, secret, store, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/documents/_search":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-x",
				"hits": map[string]any{
					"hits": []map[string]any{
						{"_id": "scan-x_2", "_source": map[string]any{
							"scanID": "scan-x", "sequenceID": 2, "contentDigest": "abc123",
						}},
					},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-x",
				"hits":       map[string]any{"hits": []map[string]any{}},
			})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	})

	payload := `{"passphrase":"` + passphrase + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".zip.enc")
	assert.Equal(t, "1", w.Header().Get("X-Document-Count"))

	// Decrypt the bundle using the same passphrase via pkg/crypt.
	crypter, err := odicrypt.New(passphrase)
	require.NoError(t, err)
	decrypted, err := crypter.Decrypt(io.NopCloser(bytes.NewReader(w.Body.Bytes())))
	require.NoError(t, err, "the bundle must decrypt with the supplied passphrase")

	plain, err := io.ReadAll(decrypted)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(plain), int64(len(plain)))
	require.NoError(t, err, "decrypted bytes must be a valid zip archive")

	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		files[f.Name] = data
	}

	// The scanned page must be present and byte-identical to what storage held.
	require.Contains(t, files, "files/scan-x_2")
	assert.Equal(t, pageContents, string(files["files/scan-x_2"]))

	// The manifest and its signature must be present.
	manifestBytes, ok := files["manifest.json"]
	require.True(t, ok, "archive must contain manifest.json")
	sigBytes, ok := files["manifest.json.sig"]
	require.True(t, ok, "archive must contain manifest.json.sig")

	// The signature must verify under the server secret.
	assert.True(t, verifyManifest([]byte(secret), manifestBytes, string(sigBytes)),
		"manifest signature must verify under the server secret")
	assert.False(t, verifyManifest([]byte("wrong-secret"), manifestBytes, string(sigBytes)),
		"manifest signature must not verify under a wrong secret")

	// The manifest must list the exported document with its digest.
	var manifest exportManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	require.Equal(t, 1, manifest.DocumentCount)
	require.Len(t, manifest.Entries, 1)
	assert.Equal(t, "scan-x", manifest.Entries[0].ScanID)
	assert.Equal(t, 2, manifest.Entries[0].SequenceID)
	assert.Equal(t, "abc123", manifest.Entries[0].ContentDigest)
}

// decryptExportArchive decrypts an export response body with the given
// passphrase and returns the files contained in the resulting zip archive keyed
// by name. It also returns the file names in archive (writing) order so callers
// can assert deterministic ordering.
func decryptExportArchive(t *testing.T, body []byte, passphrase string) (map[string][]byte, []string) {
	t.Helper()

	crypter, err := odicrypt.New(passphrase)
	require.NoError(t, err)
	decrypted, err := crypter.Decrypt(io.NopCloser(bytes.NewReader(body)))
	require.NoError(t, err, "the bundle must decrypt with the supplied passphrase")

	plain, err := io.ReadAll(decrypted)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(plain), int64(len(plain)))
	require.NoError(t, err, "decrypted bytes must be a valid zip archive")

	files := map[string][]byte{}
	order := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		files[f.Name] = data
		order = append(order, f.Name)
	}
	return files, order
}

// TestExport_ConcurrentRetrievalDeterministicOrder exercises the concurrent
// storage retrieval path with many pages spread across two scroll pages. Even
// though pages are fetched concurrently, the archive entries (and the manifest)
// must be written in deterministic scroll order.
func TestExport_ConcurrentRetrievalDeterministicOrder(t *testing.T) {
	const secret = "server-api-token"
	const passphrase = "open-sesame"

	store := newMockRWStorage()

	// First scroll page: scan-0..scan-19, second page: scan-20..scan-39.
	const pageSize = 20
	const totalDocs = 40
	for i := 0; i < totalDocs; i++ {
		store.addPage(fmt.Sprintf("scan-%02d", i), 0, []byte(fmt.Sprintf("CONTENTS-%02d", i)))
	}

	makeHits := func(from, to int) []map[string]any {
		hits := make([]map[string]any, 0, to-from)
		for i := from; i < to; i++ {
			hits = append(hits, map[string]any{
				"_id": fmt.Sprintf("scan-%02d_0", i),
				"_source": map[string]any{
					"scanID": fmt.Sprintf("scan-%02d", i), "sequenceID": 0,
					"contentDigest": fmt.Sprintf("digest-%02d", i),
				},
			})
		}
		return hits
	}

	scrollCalls := 0
	ms := exportTestServer(t, secret, store, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/documents/_search":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": makeHits(0, pageSize)},
			})
		case strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			scrollCalls++
			if scrollCalls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"_scroll_id": "scroll-1",
					"hits":       map[string]any{"hits": makeHits(pageSize, totalDocs)},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": []map[string]any{}},
			})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	})

	payload := `{"passphrase":"` + passphrase + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, fmt.Sprint(totalDocs), w.Header().Get("X-Document-Count"))

	files, order := decryptExportArchive(t, w.Body.Bytes(), passphrase)

	// Every page must be present and byte-identical to what storage held.
	for i := 0; i < totalDocs; i++ {
		name := fmt.Sprintf("files/scan-%02d_0", i)
		require.Contains(t, files, name)
		assert.Equal(t, fmt.Sprintf("CONTENTS-%02d", i), string(files[name]))
	}

	// The page files must appear in scroll order in the archive (manifest.json
	// and its signature are written after all pages).
	var fileOrder []string
	for _, name := range order {
		if strings.HasPrefix(name, "files/") {
			fileOrder = append(fileOrder, name)
		}
	}
	require.Len(t, fileOrder, totalDocs)
	for i := 0; i < totalDocs; i++ {
		assert.Equal(t, fmt.Sprintf("files/scan-%02d_0", i), fileOrder[i],
			"archive entries must be in deterministic scroll order despite concurrent retrieval")
	}

	// The manifest must list every document in the same deterministic order.
	var manifest exportManifest
	require.NoError(t, json.Unmarshal(files["manifest.json"], &manifest))
	require.Equal(t, totalDocs, manifest.DocumentCount)
	require.Len(t, manifest.Entries, totalDocs)
	for i := 0; i < totalDocs; i++ {
		assert.Equal(t, fmt.Sprintf("scan-%02d", i), manifest.Entries[i].ScanID)
		assert.Equal(t, fmt.Sprintf("digest-%02d", i), manifest.Entries[i].ContentDigest)
	}

	// The signature must still verify under the server secret.
	assert.True(t, verifyManifest([]byte(secret), files["manifest.json"], string(files["manifest.json.sig"])))
}

// TestExport_SkipsMissingPages verifies that pages absent from storage are
// skipped (not fatal) while the remaining pages export correctly and the
// manifest reflects only the retrieved documents, in deterministic order.
func TestExport_SkipsMissingPages(t *testing.T) {
	const secret = "server-api-token"
	const passphrase = "open-sesame"

	store := newMockRWStorage()
	// scan-b is intentionally NOT added to storage, so it must be skipped.
	store.addPage("scan-a", 0, []byte("A-CONTENTS"))
	store.addPage("scan-c", 0, []byte("C-CONTENTS"))

	ms := exportTestServer(t, secret, store, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/documents/_search":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits": map[string]any{"hits": []map[string]any{
					{"_id": "scan-a_0", "_source": map[string]any{"scanID": "scan-a", "sequenceID": 0, "contentDigest": "da"}},
					{"_id": "scan-b_0", "_source": map[string]any{"scanID": "scan-b", "sequenceID": 0, "contentDigest": "db"}},
					{"_id": "scan-c_0", "_source": map[string]any{"scanID": "scan-c", "sequenceID": 0, "contentDigest": "dc"}},
				}},
			})
		case strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": []map[string]any{}},
			})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	})

	payload := `{"passphrase":"` + passphrase + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	// Two documents retrieved, one skipped.
	assert.Equal(t, "2", w.Header().Get("X-Document-Count"))

	files, _ := decryptExportArchive(t, w.Body.Bytes(), passphrase)

	require.Contains(t, files, "files/scan-a_0")
	require.Contains(t, files, "files/scan-c_0")
	require.NotContains(t, files, "files/scan-b_0", "missing page must be skipped")
	assert.Equal(t, "A-CONTENTS", string(files["files/scan-a_0"]))
	assert.Equal(t, "C-CONTENTS", string(files["files/scan-c_0"]))

	var manifest exportManifest
	require.NoError(t, json.Unmarshal(files["manifest.json"], &manifest))
	require.Equal(t, 2, manifest.DocumentCount)
	require.Len(t, manifest.Entries, 2)
	// Order is preserved: scan-a then scan-c (scan-b dropped).
	assert.Equal(t, "scan-a", manifest.Entries[0].ScanID)
	assert.Equal(t, "scan-c", manifest.Entries[1].ScanID)
}

// TestExport_TooManyDocuments verifies the safety guard: an export whose
// matching set exceeds exportMaxDocuments fails cleanly with 413 instead of
// building an unbounded in-memory bundle.
func TestExport_TooManyDocuments(t *testing.T) {
	const secret = "server-api-token"
	const passphrase = "open-sesame"

	store := newMockRWStorage()

	// Emit full scroll pages of exportScrollSize hits until the cap is exceeded.
	// We never need real storage entries because the cap is checked from the hit
	// count before retrieval.
	makeFullPage := func(start int) []map[string]any {
		hits := make([]map[string]any, 0, exportScrollSize)
		for i := 0; i < exportScrollSize; i++ {
			id := start + i
			hits = append(hits, map[string]any{
				"_id":     fmt.Sprintf("scan-%d_0", id),
				"_source": map[string]any{"scanID": fmt.Sprintf("scan-%d", id), "sequenceID": 0},
			})
		}
		return hits
	}

	emitted := 0
	ms := exportTestServer(t, secret, store, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/documents/_search":
			page := makeFullPage(emitted)
			emitted += exportScrollSize
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": page},
			})
		case strings.HasPrefix(r.URL.Path, "/_search/scroll"):
			// Keep returning full pages; the handler must bail out once the
			// running total exceeds exportMaxDocuments.
			page := makeFullPage(emitted)
			emitted += exportScrollSize
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": page},
			})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	})

	payload := `{"passphrase":"` + passphrase + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "too many documents")
	// We must have stopped scrolling shortly after crossing the cap, not run away.
	assert.LessOrEqual(t, emitted, exportMaxDocuments+2*exportScrollSize)
}

// TestExport_RequiresPassphrase rejects requests without a passphrase before
// touching OpenSearch.
func TestExport_RequiresPassphrase(t *testing.T) {
	ms := exportTestServer(t, "secret", newMockRWStorage(), func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch must not be called when the passphrase is missing")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(`{"companies":["X"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestExport_RequiresAPIToken returns 503 when no server secret is configured,
// since the manifest cannot be signed.
func TestExport_RequiresAPIToken(t *testing.T) {
	ms := exportTestServer(t, "", newMockRWStorage(), func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch must not be called when API_TOKEN is unset")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader(`{"passphrase":"p"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestExport_InvalidJSON returns 400 for a malformed body.
func TestExport_InvalidJSON(t *testing.T) {
	ms := exportTestServer(t, "secret", newMockRWStorage(), func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch must not be called for invalid JSON")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
