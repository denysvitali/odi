package server

import (
	"encoding/json"
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
)

// searchTestServer creates a mockServer with a search handler wired to a fake
// OpenSearch backend. The osHandler receives the raw request that would
// normally reach OpenSearch so tests can assert on query bodies.
func searchTestServer(t *testing.T, osHandler http.HandlerFunc) *mockServer {
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
	router.POST("/api/v1/search", s.handleSearch)
	router.POST("/api/v1/search/facets", s.handleSearchFacets)
	router.GET("/api/v1/documents", s.handleGetDocuments)

	return &mockServer{Server: s, router: router}
}

// osSearchResponse builds a minimal OpenSearch search response JSON body.
func osSearchResponse(hits []map[string]any, total int) map[string]any {
	return map[string]any{
		"hits": map[string]any{
			"hits": hits,
			"total": map[string]any{
				"value":    total,
				"relation": "eq",
			},
		},
	}
}

// decodeSearchBody reads the request body sent to OpenSearch and returns it as
// a generic map so tests can inspect query structure.
func decodeSearchBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	return m
}

// ---------------------------------------------------------------------------
// Backward compatibility: search with no filters returns same results
// ---------------------------------------------------------------------------

func TestSearch_NoFilters_BackwardCompat(t *testing.T) {
	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/documents/_search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{
			{"_id": "doc-1", "_source": map[string]any{"text": "hello world"}},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	payload := `{"searchTerm":"hello","size":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify the query sent to OpenSearch is a simple query_string
	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")
	qs, ok := query["query_string"].(map[string]any)
	require.True(t, ok, "must use query_string query")
	assert.Equal(t, "hello", qs["query"])
	assert.Equal(t, "AND", qs["default_operator"])
}

// ---------------------------------------------------------------------------
// Search with company filter
// ---------------------------------------------------------------------------

func TestSearch_CompanyFilter(t *testing.T) {
	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{
			{"_id": "doc-1", "_source": map[string]any{"text": "invoice", "company": map[string]any{"name": "Swisscom"}}},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	payload := `{"searchTerm":"invoice","size":10,"companies":["Swisscom"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// When a company filter is present the query should include a bool/filter
	// clause. The exact shape depends on the implementation, but the filter
	// array must exist and mention the company name.
	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")

	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "query.bool must be present when filters are applied")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok, "bool.filter must be an array")
	assert.NotEmpty(t, filters, "filter array must not be empty")

	// The filter should reference company.name = "Swisscom"
	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "Swisscom")
}

// ---------------------------------------------------------------------------
// Search with date range filter
// ---------------------------------------------------------------------------

func TestSearch_DateRangeFilter(t *testing.T) {
	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{}, 0)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	payload := `{"searchTerm":"invoice","size":10,"dateFrom":"2024-01-01","dateTo":"2024-01-31"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")

	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "query.bool must be present for date range filter")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok, "bool.filter must be an array")
	assert.NotEmpty(t, filters)

	// Verify the range clause references the date field
	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "date")
	assert.Contains(t, string(filterJSON), "2024-01-01")
	assert.Contains(t, string(filterJSON), "2024-01-31")
}

// ---------------------------------------------------------------------------
// Search with barcode existence filter
// ---------------------------------------------------------------------------

func TestSearch_BarcodeExistenceFilter(t *testing.T) {
	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{
			{"_id": "doc-2", "_source": map[string]any{"text": "qr bill", "barcode": map[string]any{"text": "SPC..."}}},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	payload := `{"searchTerm":"bill","size":10,"hasBarcode":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")

	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "query.bool must be present for barcode filter")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok, "bool.filter must be an array")
	assert.NotEmpty(t, filters)

	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "barcode")
}

// ---------------------------------------------------------------------------
// Combined filters: company + date range
// ---------------------------------------------------------------------------

func TestSearch_CombinedFilters(t *testing.T) {
	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{}, 0)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	payload := `{"searchTerm":"invoice","size":10,"companies":["Swisscom"],"dateFrom":"2024-06-01","dateTo":"2024-06-30"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok)

	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "combined filters must produce a bool query")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok)
	// At least 2 filter clauses: company + date range
	assert.GreaterOrEqual(t, len(filters), 2, "combined filters should produce at least 2 filter clauses")

	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "Swisscom")
	assert.Contains(t, string(filterJSON), "2024-06-01")
}

// ---------------------------------------------------------------------------
// Facets endpoint returns correct aggregation buckets
// ---------------------------------------------------------------------------

func TestSearch_FacetsEndpoint(t *testing.T) {
	osResponse := map[string]any{
		"hits": map[string]any{
			"hits":  []map[string]any{},
			"total": map[string]any{"value": 0, "relation": "eq"},
		},
		"aggregations": map[string]any{
			"companies": map[string]any{
				"buckets": []map[string]any{
					{"key": "Swisscom", "doc_count": 12},
					{"key": "SBB", "doc_count": 5},
				},
			},
			"date_histogram": map[string]any{
				"buckets": []map[string]any{},
			},
			"barcode_count": map[string]any{
				"doc_count": 8,
			},
		},
	}

	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(osResponse)
	})

	payload := `{"searchTerm":"*"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search/facets", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify the request body includes aggs (size 0 for aggregations only)
	assert.Equal(t, float64(0), capturedBody["size"])
	_, hasAggs := capturedBody["aggs"]
	assert.True(t, hasAggs, "facets request must include aggs in the OpenSearch request")

	// Verify the response includes aggregation buckets
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))

	aggs, ok := result["aggregations"].(map[string]any)
	require.True(t, ok, "response must include aggregations")
	_, hasCompanies := aggs["companies"]
	assert.True(t, hasCompanies, "aggregations must include companies facet")
	_, hasDateHist := aggs["date_histogram"]
	assert.True(t, hasDateHist, "aggregations must include date_histogram facet")
	_, hasBarcodeCount := aggs["barcode_count"]
	assert.True(t, hasBarcodeCount, "aggregations must include barcode_count facet")
}

// ---------------------------------------------------------------------------
// Empty filter arrays are handled gracefully
// ---------------------------------------------------------------------------

func TestSearch_EmptyFilterArrays(t *testing.T) {
	var capturedBody map[string]any

	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{
			{"_id": "doc-1", "_source": map[string]any{"text": "test"}},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Empty companies array — no effective filters
	payload := `{"searchTerm":"test","size":10,"companies":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// With empty filters the query should degrade to a simple query_string
	// without a bool wrapper, or if a bool wrapper is used, the filter array
	// should be empty or absent.
	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")

	// Either the query is a plain query_string (no bool) or the bool filter
	// is empty — both are acceptable. The key assertion is that we don't
	// get a 500 error and the response is valid.
	if boolQ, hasBool := query["bool"].(map[string]any); hasBool {
		if filters, hasFilter := boolQ["filter"].([]any); hasFilter {
			assert.Empty(t, filters, "empty filter arrays should not produce filter clauses")
		}
	}
}

// ---------------------------------------------------------------------------
// handleGetDocuments: date range filters via query parameters
// ---------------------------------------------------------------------------

func TestGetDocuments_DateRangeFilter(t *testing.T) {
	var capturedBody map[string]any

	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{
			{"_id": "doc-1", "_source": map[string]any{"text": "test"}},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer osSrv.Close()

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/documents", s.handleGetDocuments)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?date_from=2024-03-01&date_to=2024-03-31", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify that date filters are present in the query body
	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query must be present when date filters are set")

	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "must use bool query for date filters")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok, "bool.filter must be an array")
	assert.NotEmpty(t, filters)

	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "2024-03-01")
}

// ---------------------------------------------------------------------------
// handleGetDocuments: no filters returns all documents
// ---------------------------------------------------------------------------

func TestGetDocuments_NoFilters(t *testing.T) {
	var capturedBody map[string]any

	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{
			{"_id": "doc-1", "_source": map[string]any{"text": "test"}},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer osSrv.Close()

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/documents", s.handleGetDocuments)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Without date filters, the body should NOT contain a bool query
	_, hasQuery := capturedBody["query"]
	// It's acceptable for there to be no query at all (match_all implicit)
	// or a match_all. The important thing is no error occurs.
	if hasQuery {
		query := capturedBody["query"].(map[string]any)
		_, hasBool := query["bool"]
		assert.False(t, hasBool, "no filters should not produce a bool query")
	}
}

// ---------------------------------------------------------------------------
// handleGetDocuments: invalid date_from returns 400
// ---------------------------------------------------------------------------

func TestGetDocuments_InvalidDateFrom(t *testing.T) {
	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for invalid input")
	}))
	defer osSrv.Close()

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/documents", s.handleGetDocuments)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?date_from=not-a-date", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// handleGetDocuments: invalid date_to returns 400
// ---------------------------------------------------------------------------

func TestGetDocuments_InvalidDateTo(t *testing.T) {
	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for invalid input")
	}))
	defer osSrv.Close()

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/documents", s.handleGetDocuments)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents?date_to=2024/13/99", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Search: size exceeds maximum returns 400
// ---------------------------------------------------------------------------

func TestSearch_SizeExceedsMax(t *testing.T) {
	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for oversized request")
	})

	payload := `{"searchTerm":"test","size":9999}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Search: invalid JSON returns 400
// ---------------------------------------------------------------------------

func TestSearch_InvalidJSON(t *testing.T) {
	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for invalid JSON")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Search: OpenSearch error returns 500
// ---------------------------------------------------------------------------

func TestSearch_OpenSearchError(t *testing.T) {
	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"internal","reason":"something broke"}}`))
	})

	payload := `{"searchTerm":"test","size":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
