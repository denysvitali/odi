package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v5"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchAndFacets_QueryParity(t *testing.T) {
	for _, term := range []string{"invoice AND paid", "  ", ""} {
		t.Run("term="+term, func(t *testing.T) {
			var queries []any
			ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				body := decodeSearchBody(t, r)
				queries = append(queries, body["query"])
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(osSearchResponse(nil, 0))
			})
			payload, err := json.Marshal(map[string]any{
				"searchTerm": term,
				"companies":  []string{"Example"},
				"dateFrom":   "2025-01-01",
				"dateTo":     "2025-12-31",
				"hasBarcode": true,
				"title":      "Statement",
				"docTypes":   []string{"invoice"},
				"tags":       []string{"paid"},
			})
			require.NoError(t, err)
			for _, path := range []string{"/api/v1/search", "/api/v1/search/facets"} {
				w := httptest.NewRecorder()
				ms.router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(payload))))
				require.Equal(t, http.StatusOK, w.Code)
			}
			require.Len(t, queries, 2)
			assert.Equal(t, queries[0], queries[1], "facet counts must use the same query as result hits")
		})
	}
}

var rawSearchEndpoints = []struct {
	name   string
	method string
	path   string
	body   string
	scroll string
}{
	{"search", http.MethodPost, "/api/v1/search", `{"searchTerm":"statement","size":7}`, "600000ms"},
	{"facets", http.MethodPost, "/api/v1/search/facets", `{"searchTerm":"statement"}`, ""},
	{"documents", http.MethodGet, "/api/v1/documents?size=7", "", "600000ms"},
}

func TestRawSearchEndpoints_PreserveResponseAndScroll(t *testing.T) {
	// Unknown OpenSearch fields and whitespace survive raw response streaming.
	const upstream = `{"hits":{"hits":[],"total":{"value":0,"relation":"eq"}}, "extra":{"kept":true}}`
	for _, endpoint := range rawSearchEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/documents/_search", r.URL.Path)
				assert.Equal(t, endpoint.scroll, r.URL.Query().Get("scroll"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(upstream))
			})
			w := httptest.NewRecorder()
			ms.router.ServeHTTP(w, httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body)))
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, upstream, w.Body.String())
		})
	}
}

func TestRawSearchEndpoints_UpstreamFailureIsGeneric(t *testing.T) {
	for _, endpoint := range rawSearchEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"type":"query_error","reason":"private upstream detail"},"status":400}`))
			})
			w := httptest.NewRecorder()
			ms.router.ServeHTTP(w, httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body)))
			require.Equal(t, http.StatusInternalServerError, w.Code)
			assert.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())
		})
	}
}

type searchRoundTripFunc func(*http.Request) (*http.Response, error)

func (f searchRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestRawSearchEndpoints_PropagateCancellation(t *testing.T) {
	for _, endpoint := range rawSearchEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := false
			client, err := opensearchapi.NewClient(opensearchapi.Config{
				Client: opensearch.Config{
					Addresses:    []string{"http://opensearch.test"},
					DisableRetry: true,
					Transport: searchRoundTripFunc(func(r *http.Request) (*http.Response, error) {
						called = true
						cancel()
						assert.ErrorIs(t, r.Context().Err(), context.Canceled)
						return nil, r.Context().Err()
					}),
				},
			})
			require.NoError(t, err)
			gin.SetMode(gin.TestMode)
			router := gin.New()
			s := &Server{osClient: client, osIndex: "documents"}
			router.POST("/api/v1/search", s.handleSearch)
			router.POST("/api/v1/search/facets", s.handleSearchFacets)
			router.GET("/api/v1/documents", s.handleGetDocuments)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body)).WithContext(ctx)
			router.ServeHTTP(w, req)
			assert.True(t, called)
			require.Equal(t, http.StatusInternalServerError, w.Code)
			assert.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())
		})
	}
}
