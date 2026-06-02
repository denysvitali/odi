package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/odi/pkg/models"
)

// remindersTestServer wires the reminders handler to a fake OpenSearch backend
// so tests can assert on the query body OpenSearch receives and the projected
// response shape. Mirrors searchTestServer in search_test.go.
func remindersTestServer(t *testing.T, osHandler http.HandlerFunc) *mockServer {
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
	router.GET("/api/v1/reminders", s.handleReminders)

	return &mockServer{Server: s, router: router}
}

// ---------------------------------------------------------------------------
// Range query shape: dates array filtered by now..now+days
// ---------------------------------------------------------------------------

func TestReminders_RangeQueryShape(t *testing.T) {
	var capturedBody map[string]any

	ms := remindersTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/documents/_search", r.URL.Path)
		capturedBody = decodeSearchBody(t, r)
		resp := osSearchResponse([]map[string]any{}, 0)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reminders", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	query, ok := capturedBody["query"].(map[string]any)
	require.True(t, ok, "query field must be present")
	boolQ, ok := query["bool"].(map[string]any)
	require.True(t, ok, "reminders query must be a bool query")
	filters, ok := boolQ["filter"].([]any)
	require.True(t, ok, "bool.filter must be an array")
	require.NotEmpty(t, filters)

	// First filter must be a range over the mapped `dates` array with gte/lte.
	filterJSON, _ := json.Marshal(filters)
	assert.Contains(t, string(filterJSON), "range")
	assert.Contains(t, string(filterJSON), "dates")
	assert.Contains(t, string(filterJSON), "gte")
	assert.Contains(t, string(filterJSON), "lte")

	// The docType narrowing must be present too (invoice/insurance/contract).
	assert.Contains(t, string(filterJSON), "docType.keyword")
	assert.Contains(t, string(filterJSON), "invoice")

	// Default window of 90 days is echoed back in the response envelope.
	var result remindersResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, defaultReminderWindowDays, result.Days)
}

// ---------------------------------------------------------------------------
// Custom window via ?days= overrides the default
// ---------------------------------------------------------------------------

func TestReminders_CustomDaysWindow(t *testing.T) {
	ms := remindersTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := osSearchResponse([]map[string]any{}, 0)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reminders?days=30", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result remindersResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 30, result.Days)
}

// ---------------------------------------------------------------------------
// Invalid ?days= returns 400 and never calls OpenSearch
// ---------------------------------------------------------------------------

func TestReminders_InvalidDays(t *testing.T) {
	ms := remindersTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("OpenSearch should not be called for invalid days")
	})

	for _, raw := range []string{"abc", "-5", "0"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reminders?days="+raw, nil)
		w := httptest.NewRecorder()
		ms.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "days=%q should be rejected", raw)
	}
}

// ---------------------------------------------------------------------------
// Soonest-date selection and KeyFact enrichment
// ---------------------------------------------------------------------------

func TestReminders_SoonestDateAndEnrichment(t *testing.T) {
	now := time.Now().UTC()
	soon := now.AddDate(0, 0, 5).Format(time.RFC3339)
	later := now.AddDate(0, 0, 40).Format(time.RFC3339)
	past := now.AddDate(0, 0, -10).Format(time.RFC3339)

	ms := remindersTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := osSearchResponse([]map[string]any{
			{
				"_id": "scan-1_0",
				"_source": map[string]any{
					"title":   "Electricity bill",
					"docType": "invoice",
					"company": map[string]any{"name": "EWZ"},
					// Out of order, plus a past date that must be ignored.
					"dates": []string{later, past, soon},
					"keyFacts": []map[string]any{
						{"label": "Amount due", "value": "CHF 120.50"},
					},
				},
			},
		}, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reminders", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result remindersResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.Reminders, 1)

	r := result.Reminders[0]
	assert.Equal(t, "scan-1_0", r.ID)
	assert.Equal(t, "Electricity bill", r.Title)
	assert.Equal(t, "invoice", r.DocType)
	assert.Equal(t, "EWZ", r.Company)
	assert.Equal(t, "CHF 120.50", r.AmountDue)

	// The soonest future date (soon), not later/past, must be chosen.
	picked, err := time.Parse(time.RFC3339, r.DueDate)
	require.NoError(t, err)
	expected, err := time.Parse(time.RFC3339, soon)
	require.NoError(t, err)
	assert.WithinDuration(t, expected, picked, time.Second)
}

// ---------------------------------------------------------------------------
// Ascending sort by due date across multiple documents
// ---------------------------------------------------------------------------

func TestReminders_AscendingSort(t *testing.T) {
	now := time.Now().UTC()
	d3 := now.AddDate(0, 0, 60).Format(time.RFC3339)
	d1 := now.AddDate(0, 0, 3).Format(time.RFC3339)
	d2 := now.AddDate(0, 0, 20).Format(time.RFC3339)

	ms := remindersTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := osSearchResponse([]map[string]any{
			{"_id": "c", "_source": map[string]any{"title": "C", "docType": "contract", "dates": []string{d3}}},
			{"_id": "a", "_source": map[string]any{"title": "A", "docType": "invoice", "dates": []string{d1}}},
			{"_id": "b", "_source": map[string]any{"title": "B", "docType": "insurance", "dates": []string{d2}}},
		}, 3)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reminders", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result remindersResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.Reminders, 3)

	assert.Equal(t, "a", result.Reminders[0].ID)
	assert.Equal(t, "b", result.Reminders[1].ID)
	assert.Equal(t, "c", result.Reminders[2].ID)

	assert.True(t, result.Reminders[0].DueDate <= result.Reminders[1].DueDate)
	assert.True(t, result.Reminders[1].DueDate <= result.Reminders[2].DueDate)
}

// ---------------------------------------------------------------------------
// OpenSearch error propagates as 500
// ---------------------------------------------------------------------------

func TestReminders_OpenSearchError(t *testing.T) {
	ms := remindersTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"internal","reason":"boom"}}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reminders", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---------------------------------------------------------------------------
// Unit: soonestFutureDate selection logic
// ---------------------------------------------------------------------------

func TestSoonestFutureDate(t *testing.T) {
	now := time.Now().UTC()
	until := now.AddDate(0, 0, 90)

	t.Run("no dates in window", func(t *testing.T) {
		_, ok := soonestFutureDate([]time.Time{now.AddDate(0, 0, -1), now.AddDate(0, 0, 200)}, now, until)
		assert.False(t, ok)
	})

	t.Run("picks earliest in window", func(t *testing.T) {
		early := now.AddDate(0, 0, 2)
		late := now.AddDate(0, 0, 50)
		got, ok := soonestFutureDate([]time.Time{late, early}, now, until)
		require.True(t, ok)
		assert.WithinDuration(t, early, got, time.Second)
	})
}

// ---------------------------------------------------------------------------
// Unit: keyFactValue label matching (case-insensitive, first match wins)
// ---------------------------------------------------------------------------

func TestKeyFactValue(t *testing.T) {
	facts := []models.KeyFact{
		{Label: "IBAN", Value: "CH00"},
		{Label: "Amount Due", Value: "CHF 100"},
	}

	assert.Equal(t, "CHF 100", keyFactValue(facts, amountDueFactLabels))
	assert.Equal(t, "", keyFactValue(facts, dueDateFactLabels))
	assert.Equal(t, "", keyFactValue(nil, amountDueFactLabels))
}
