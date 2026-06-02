package server

import (
	"encoding/csv"
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

// insightsTestServer wires the insights handlers to a fake OpenSearch backend
// that serves a configurable sequence of scroll pages.
func insightsTestServer(t *testing.T, pages []map[string]any) *mockServer {
	t.Helper()

	var page int
	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/documents/_search":
			_ = json.NewEncoder(w).Encode(pages[0])
			page = 1
		case r.Method == http.MethodPost && r.URL.Path == "/_search/scroll":
			if page < len(pages) {
				_ = json.NewEncoder(w).Encode(pages[page])
				page++
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"_scroll_id": "scroll-1",
					"hits":       map[string]any{"hits": []any{}},
				})
			}
		case r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{"succeeded": true})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(osSrv.Close)

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/insights", s.handleInsights)
	router.GET("/api/v1/insights.csv", s.handleInsightsCSV)

	return &mockServer{Server: s, router: router}
}

// scrollPage builds a single scroll page response carrying the supplied hit
// sources.
func scrollPage(sources ...map[string]any) map[string]any {
	hits := make([]map[string]any, 0, len(sources))
	for _, src := range sources {
		hits = append(hits, map[string]any{"_source": src})
	}
	return map[string]any{
		"_scroll_id": "scroll-1",
		"hits":       map[string]any{"hits": hits},
	}
}

func docSource(company string, date string, docType string, label string, value string) map[string]any {
	return map[string]any{
		"company":  map[string]any{"name": company},
		"date":     date,
		"docType":  docType,
		"keyFacts": []map[string]any{{"label": label, "value": value}},
	}
}

// ---------------------------------------------------------------------------
// parseAmount unit tests
// ---------------------------------------------------------------------------

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		want     float64
		currency string
		ok       bool
	}{
		{"chf swiss thousands", "CHF 1'240.00", 1240.00, "CHF", true},
		{"eur dot decimal", "EUR 99.90", 99.90, "EUR", true},
		{"comma decimal", "1240,50", 1240.50, "", true},
		{"comma thousands dot decimal", "USD 1,240.00", 1240.00, "USD", true},
		{"dot thousands comma decimal", "1.240,00", 1240.00, "", true},
		{"plain integer", "500", 500, "", true},
		{"no number", "n/a", 0, "", false},
		{"empty", "", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, currency, ok := parseAmount(tt.in)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.InDelta(t, tt.want, got, 0.001)
				assert.Equal(t, tt.currency, currency)
			}
		})
	}
}

func TestAmountLabelRegexp(t *testing.T) {
	assert.True(t, amountLabelRegexp.MatchString("Amount due"))
	assert.True(t, amountLabelRegexp.MatchString("Total paid"))
	assert.True(t, amountLabelRegexp.MatchString("Balance"))
	assert.False(t, amountLabelRegexp.MatchString("Invoice number"))
	assert.False(t, amountLabelRegexp.MatchString("IBAN"))
}

// ---------------------------------------------------------------------------
// handleInsights: aggregation across multiple scroll pages
// ---------------------------------------------------------------------------

func TestInsights_AggregatesAcrossPages(t *testing.T) {
	pages := []map[string]any{
		scrollPage(
			docSource("Swisscom", "2024-03-01T00:00:00Z", "invoice", "Amount due", "CHF 100.00"),
			docSource("Swisscom", "2024-06-01T00:00:00Z", "invoice", "Total", "CHF 50.00"),
		),
		scrollPage(
			docSource("SBB", "2023-01-01T00:00:00Z", "receipt", "Amount paid", "CHF 25.50"),
		),
	}

	ms := insightsTestServer(t, pages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp InsightsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "CHF", resp.Currency)
	assert.Equal(t, 3, resp.Count)
	assert.InDelta(t, 175.50, resp.Total, 0.001)

	require.Len(t, resp.ByCompany, 2)
	// Swisscom has the highest total (150) so it sorts first.
	assert.Equal(t, "Swisscom", resp.ByCompany[0].Name)
	assert.InDelta(t, 150.00, resp.ByCompany[0].Total, 0.001)
	assert.Equal(t, 2, resp.ByCompany[0].Count)

	require.Len(t, resp.ByYear, 2)
	// Years are chronological.
	assert.Equal(t, "2023", resp.ByYear[0].Name)
	assert.Equal(t, "2024", resp.ByYear[1].Name)
	assert.InDelta(t, 150.00, resp.ByYear[1].Total, 0.001)

	require.Len(t, resp.ByDocType, 2)
}

// ---------------------------------------------------------------------------
// handleInsights: year filter narrows the results
// ---------------------------------------------------------------------------

func TestInsights_YearFilter(t *testing.T) {
	pages := []map[string]any{
		scrollPage(
			docSource("Swisscom", "2024-03-01T00:00:00Z", "invoice", "Amount due", "CHF 100.00"),
			docSource("SBB", "2023-01-01T00:00:00Z", "receipt", "Total", "CHF 25.00"),
		),
	}

	ms := insightsTestServer(t, pages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights?year=2024", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp InsightsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, 1, resp.Count)
	assert.InDelta(t, 100.00, resp.Total, 0.001)
	require.Len(t, resp.ByYear, 1)
	assert.Equal(t, "2024", resp.ByYear[0].Name)
}

// ---------------------------------------------------------------------------
// effectiveDateRange: year filter folds into / intersects explicit bounds
// ---------------------------------------------------------------------------

func TestEffectiveDateRange(t *testing.T) {
	tests := []struct {
		name    string
		year    string
		from    string
		to      string
		wantGte string
		wantLte string
		wantOk  bool
	}{
		{"year only", "2024", "", "", "2024-01-01", "2024-12-31", true},
		{"no filters", "", "", "", "", "", true},
		{"explicit bounds only", "", "2024-03-01", "2024-09-30", "2024-03-01", "2024-09-30", true},
		{"year tighter than bounds", "2024", "2020-01-01", "2030-12-31", "2024-01-01", "2024-12-31", true},
		{"bounds tighter than year", "2024", "2024-06-01", "2024-06-30", "2024-06-01", "2024-06-30", true},
		{"year with one-sided lower bound", "2024", "2024-07-01", "", "2024-07-01", "2024-12-31", true},
		{"year with one-sided upper bound", "2024", "", "2024-04-30", "2024-01-01", "2024-04-30", true},
		{"impossible overlap", "2024", "", "2023-01-01", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gte, lte, ok := effectiveDateRange(tt.year, tt.from, tt.to)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.wantGte, gte)
				assert.Equal(t, tt.wantLte, lte)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleInsights: the year filter is pushed down into the OpenSearch query
// ---------------------------------------------------------------------------

func TestInsights_YearFilterPushedDownToQuery(t *testing.T) {
	var searchBody string
	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/documents/_search":
			b, _ := io.ReadAll(r.Body)
			searchBody = string(b)
			_ = json.NewEncoder(w).Encode(scrollPage(
				docSource("Swisscom", "2024-03-01T00:00:00Z", "invoice", "Amount due", "CHF 100.00"),
			))
		case r.Method == http.MethodPost && r.URL.Path == "/_search/scroll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"_scroll_id": "scroll-1",
				"hits":       map[string]any{"hits": []any{}},
			})
		case r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{"succeeded": true})
		default:
			t.Fatalf("unexpected OpenSearch request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(osSrv.Close)

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/insights", s.handleInsights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights?year=2024", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// The query must carry a date range derived from the year, so a year-scoped
	// request is filtered server-side rather than by scrolling the whole corpus.
	assert.Contains(t, searchBody, "range")
	assert.Contains(t, searchBody, "2024-01-01")
	assert.Contains(t, searchBody, "2024-12-31")
}

// ---------------------------------------------------------------------------
// handleInsights: an impossible year/date overlap short-circuits without a query
// ---------------------------------------------------------------------------

func TestInsights_ImpossibleRangeSkipsQuery(t *testing.T) {
	osSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("OpenSearch should not be called for an empty date range: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(osSrv.Close)

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/insights", s.handleInsights)

	// year=2024 cannot overlap a date_to of 2023-01-01.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights?year=2024&date_to=2023-01-01", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp InsightsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.ByCompany)
	assert.Empty(t, resp.ByYear)
	assert.Empty(t, resp.ByDocType)
}

// ---------------------------------------------------------------------------
// handleInsights: invalid year returns 400
// ---------------------------------------------------------------------------

func TestInsights_InvalidYear(t *testing.T) {
	osSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for invalid year")
	}))
	defer osSrv.Close()

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{osSrv.URL}},
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{e: router, osClient: osClient, osIndex: "documents"}
	router.GET("/api/v1/insights", s.handleInsights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights?year=notayear", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// handleInsightsCSV: emits headers and rows
// ---------------------------------------------------------------------------

func TestInsightsCSV_HeadersAndRows(t *testing.T) {
	pages := []map[string]any{
		scrollPage(
			docSource("Swisscom", "2024-03-01T00:00:00Z", "invoice", "Amount due", "CHF 100.00"),
		),
	}

	ms := insightsTestServer(t, pages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights.csv", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "insights.csv")

	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, records)

	assert.Equal(t, []string{"dimension", "name", "total", "count", "currency"}, records[0])

	// At least one company row, one year row, one docType row plus the header.
	var hasCompany, hasYear, hasDocType bool
	for _, row := range records[1:] {
		switch row[0] {
		case "company":
			hasCompany = true
			assert.Equal(t, "Swisscom", row[1])
			assert.Equal(t, "100.00", row[2])
			assert.Equal(t, "CHF", row[4])
		case "year":
			hasYear = true
			assert.Equal(t, "2024", row[1])
		case "docType":
			hasDocType = true
			assert.Equal(t, "invoice", row[1])
		}
	}
	assert.True(t, hasCompany, "expected a company row")
	assert.True(t, hasYear, "expected a year row")
	assert.True(t, hasDocType, "expected a docType row")
}

// ---------------------------------------------------------------------------
// handleInsights: documents without parseable amounts are skipped
// ---------------------------------------------------------------------------

func TestInsights_SkipsUnparseableAmounts(t *testing.T) {
	pages := []map[string]any{
		scrollPage(
			docSource("Swisscom", "2024-03-01T00:00:00Z", "invoice", "Reference", "RF18 5390 0754 7034"),
			docSource("Swisscom", "2024-03-02T00:00:00Z", "invoice", "Amount due", "CHF 42.00"),
		),
	}

	ms := insightsTestServer(t, pages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp InsightsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, 1, resp.Count)
	assert.InDelta(t, 42.00, resp.Total, 0.001)
}
