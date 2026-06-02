package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// similarTestServer wires the similar-documents handler to a fake OpenSearch
// backend. The osHandler receives the raw HTTP requests (both the document GET
// and the _search) so tests can assert on the query body and inject responses.
func similarTestServer(t *testing.T, osHandler http.HandlerFunc) *mockServer {
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
	}
	router.GET("/api/v1/documents/:id/similar", s.handleSimilarDocuments)

	return &mockServer{Server: s, router: router}
}

// ---------------------------------------------------------------------------
// Happy path: builds a more_like_this query and projects the hits.
// ---------------------------------------------------------------------------

func TestSimilar_ReturnsProjectedHits(t *testing.T) {
	var capturedSearch map[string]any

	ms := similarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		// Source document fetch.
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_index": "documents",
				"_id":    "abc_1",
				"found":  true,
				"_source": map[string]any{
					"text":   "monthly invoice from Swisscom",
					"scanID": "abc",
				},
			})
		// more_like_this search.
		case http.MethodPost:
			capturedSearch = decodeSearchBody(t, r)
			resp := osSearchResponse([]map[string]any{
				{
					"_id": "def_1",
					"_source": map[string]any{
						"title":   "Invoice February",
						"date":    "2024-02-01",
						"docType": "invoice",
						"scanID":  "def",
						"company": map[string]any{"name": "Swisscom"},
					},
				},
			}, 1)
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/abc_1/similar", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Assert the query shape: bool.must -> more_like_this over the expected
	// fields, seeded by {_index,_id}, plus must_not on the source id + scanID.
	query, ok := capturedSearch["query"].(map[string]any)
	require.True(t, ok, "query field must be present")
	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "must use a bool query")

	must, ok := boolQ["must"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, must)
	mlt, ok := must[0].(map[string]any)["more_like_this"].(map[string]any)
	require.True(t, ok, "must clause must be a more_like_this query")
	assert.Equal(t, []any{"text", "title", "company.name"}, mlt["fields"])
	assert.Equal(t, float64(1), mlt["min_term_freq"])
	assert.Equal(t, float64(1), mlt["min_doc_freq"])

	mltJSON, _ := json.Marshal(mlt["like"])
	assert.Contains(t, string(mltJSON), `"_id":"abc_1"`)
	assert.Contains(t, string(mltJSON), `"_index":"documents"`)

	mustNotJSON, _ := json.Marshal(boolQ["must_not"])
	assert.Contains(t, string(mustNotJSON), "abc_1", "must_not should exclude the source id")
	assert.Contains(t, string(mustNotJSON), "scanID", "must_not should exclude same-scan siblings")
	assert.Contains(t, string(mustNotJSON), "abc", "must_not should reference the source scanID")

	assert.Equal(t, float64(similarSize), capturedSearch["size"])

	// Assert the projected response.
	var body similarResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Documents, 1)
	assert.Equal(t, "def_1", body.Documents[0].ID)
	assert.Equal(t, "Invoice February", body.Documents[0].Title)
	assert.Equal(t, "2024-02-01", body.Documents[0].Date)
	assert.Equal(t, "invoice", body.Documents[0].DocType)
	assert.Equal(t, "Swisscom", body.Documents[0].Company)
}

// ---------------------------------------------------------------------------
// The source document GET is _source-restricted to scanID so the (potentially
// huge) OCR text is never transferred. A document whose projected source has no
// usable seed still runs the more_like_this search (the source doc is excluded
// from the results anyway) and returns whatever the search yields — here, none.
// ---------------------------------------------------------------------------

func TestSimilar_RestrictsGetSourceToScanID(t *testing.T) {
	var getSourceIncludes string
	var searched bool

	ms := similarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			getSourceIncludes = r.URL.Query().Get("_source_includes")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_index":  "documents",
				"_id":     "abc_1",
				"found":   true,
				"_source": map[string]any{"scanID": "abc"},
			})
		case http.MethodPost:
			searched = true
			_ = json.NewEncoder(w).Encode(osSearchResponse([]map[string]any{}, 0))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/abc_1/similar", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// The GET must only pull scanID, not the full (possibly megabyte) source.
	assert.Equal(t, "scanID", getSourceIncludes, "document GET must restrict _source to scanID")
	assert.True(t, searched, "the more_like_this search should still run")

	var body similarResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Empty(t, body.Documents)
}

// ---------------------------------------------------------------------------
// Invalid document id returns 400 and never reaches OpenSearch.
// ---------------------------------------------------------------------------

func TestSimilar_InvalidID(t *testing.T) {
	ms := similarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for an invalid id")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/not-a-valid-id/similar", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Missing source document returns 404.
// ---------------------------------------------------------------------------

func TestSimilar_NotFound(t *testing.T) {
	ms := similarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"found": false})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/abc_1/similar", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// A _search error surfaces as 500.
// ---------------------------------------------------------------------------

func TestSimilar_SearchError(t *testing.T) {
	ms := similarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_index": "documents",
				"_id":    "abc_1",
				"found":  true,
				"_source": map[string]any{
					"text":   "some text",
					"scanID": "abc",
				},
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"internal","reason":"boom"}}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/abc_1/similar", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
