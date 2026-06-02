package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/denysvitali/odi/pkg/models"
)

// defaultReminderWindowDays is the look-ahead window (in days) used when the
// caller does not supply ?days=.
const defaultReminderWindowDays = 90

// maxReminderWindowDays caps how far into the future we look so a bogus ?days=
// value cannot turn the range query into an unbounded scan.
const maxReminderWindowDays = 3650

// reminderSearchSize bounds the number of documents pulled for the upcoming
// view. Reminders are a curated, near-term list, not a paginated archive.
const reminderSearchSize = 200

// reminderDocTypes narrows the upcoming view to the document kinds that
// actually carry actionable deadlines (bills, renewals, policies). Mirrors the
// docType.keyword faceting used by buildDocTypeTagFilters.
var reminderDocTypes = []string{"invoice", "insurance", "contract"}

// dueDateFactLabels are the KeyFact labels (lower-cased) we treat as the
// human-readable due date for a document.
var dueDateFactLabels = []string{"due date", "payment due", "renewal date", "expiry date", "expiration date"}

// amountDueFactLabels are the KeyFact labels (lower-cased) we treat as the
// amount owed for a document.
var amountDueFactLabels = []string{"amount due", "amount", "total", "total due", "balance due"}

// Reminder is the projected line shown in the "Upcoming" deadlines view. It is
// derived from a document's mapped dates and KeyFacts; no LLM call is involved.
type Reminder struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	DueDate   string `json:"dueDate"`
	DocType   string `json:"docType,omitempty"`
	Company   string `json:"company,omitempty"`
	AmountDue string `json:"amountDue,omitempty"`
}

// remindersResponse is the JSON envelope returned by handleReminders.
type remindersResponse struct {
	Reminders []Reminder `json:"reminders"`
	Days      int        `json:"days"`
}

// reminderHit is the minimal OpenSearch hit projection needed to build a
// reminder line.
type reminderHit struct {
	ID     string          `json:"_id"`
	Source models.Document `json:"_source"`
}

type reminderSearchResponse struct {
	Hits struct {
		Hits []reminderHit `json:"hits"`
	} `json:"hits"`
}

// handleReminders returns documents whose soonest future date falls inside the
// look-ahead window (now .. now+days, default 90), projected as a flat list of
// reminders sorted ascending by due date. The query is narrowed to the docTypes
// that carry actionable deadlines (invoice/insurance/contract).
func (s *Server) handleReminders(c *gin.Context) {
	days := defaultReminderWindowDays
	if raw := strings.TrimSpace(c.Query("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be a positive integer"})
			return
		}
		days = parsed
	}
	if days > maxReminderWindowDays {
		days = maxReminderWindowDays
	}

	now := time.Now().UTC()
	until := now.AddDate(0, 0, days)

	query := buildReminderQuery(now, until)
	searchContent := map[string]any{
		"size":    reminderSearchSize,
		"query":   query,
		"_source": []string{"title", "dates", "docType", "company", "companies", "keyFacts"},
	}

	jsonBody, marshalErr := json.Marshal(searchContent)
	if marshalErr != nil {
		log.Errorf("unable to marshal reminders body (days=%d): %v", days, marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	searchResp, err := s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
	})
	if err != nil {
		log.Errorf("unable to perform reminders search (days=%d): %v", days, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer searchResp.Inspect().Response.Body.Close()

	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("reminders search returned error (days=%d): %s", days, searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	var parsed reminderSearchResponse
	if err := json.NewDecoder(searchResp.Inspect().Response.Body).Decode(&parsed); err != nil {
		log.Errorf("unable to decode reminders response (days=%d): %v", days, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	reminders := buildReminders(parsed.Hits.Hits, now, until)

	c.JSON(http.StatusOK, remindersResponse{
		Reminders: reminders,
		Days:      days,
	})
}

// buildReminderQuery composes the bool query used by the upcoming view: a range
// filter on the mapped `dates` array (now .. until) narrowed by the actionable
// docTypes. The range matches if ANY date in the array falls in the window;
// per-hit selection of the soonest future date happens in buildReminders.
func buildReminderQuery(now, until time.Time) map[string]any {
	filters := []map[string]any{
		{
			"range": map[string]any{
				"dates": map[string]any{
					"gte": now.Format(time.RFC3339),
					"lte": until.Format(time.RFC3339),
				},
			},
		},
	}

	if docTypeFilters := buildDocTypeTagFilters(reminderDocTypes, nil); len(docTypeFilters) > 0 {
		filters = append(filters, docTypeFilters...)
	}

	return map[string]any{
		"bool": map[string]any{
			"filter": filters,
		},
	}
}

// buildReminders projects raw hits into the sorted reminder list. For each hit
// it picks the soonest date strictly inside [now, until], enriches the line
// with the matching due-date / amount-due KeyFacts, and finally sorts the whole
// list ascending by due date.
func buildReminders(hits []reminderHit, now, until time.Time) []Reminder {
	reminders := make([]Reminder, 0, len(hits))

	for _, hit := range hits {
		due, ok := soonestFutureDate(hit.Source.Dates, now, until)
		if !ok {
			continue
		}

		reminder := Reminder{
			ID:        hit.ID,
			Title:     hit.Source.Title,
			DueDate:   due.Format(time.RFC3339),
			DocType:   hit.Source.DocType,
			Company:   reminderCompany(hit.Source),
			AmountDue: keyFactValue(hit.Source.KeyFacts, amountDueFactLabels),
		}

		// Prefer an explicitly extracted due-date KeyFact for the displayed
		// label when present, falling back to the soonest mapped date above.
		if label := keyFactValue(hit.Source.KeyFacts, dueDateFactLabels); label != "" {
			if parsed, perr := parseFlexibleDate(label); perr == nil {
				reminder.DueDate = parsed.Format(time.RFC3339)
			}
		}

		reminders = append(reminders, reminder)
	}

	sort.SliceStable(reminders, func(i, j int) bool {
		return reminders[i].DueDate < reminders[j].DueDate
	})

	return reminders
}

// soonestFutureDate returns the earliest date in dates that is within
// [now, until]. It reports ok=false when no date falls in the window.
func soonestFutureDate(dates []time.Time, now, until time.Time) (time.Time, bool) {
	var soonest time.Time
	found := false
	for _, d := range dates {
		du := d.UTC()
		if du.Before(now) || du.After(until) {
			continue
		}
		if !found || du.Before(soonest) {
			soonest = du
			found = true
		}
	}
	return soonest, found
}

// keyFactValue returns the value of the first KeyFact whose (lower-cased) label
// matches one of the supplied candidate labels. Returns "" when none match.
func keyFactValue(facts []models.KeyFact, labels []string) string {
	for _, label := range labels {
		for _, f := range facts {
			if strings.EqualFold(strings.TrimSpace(f.Label), label) {
				if v := strings.TrimSpace(f.Value); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// reminderCompany resolves the best company name for a document, preferring the
// primary Company and falling back to the first entry in Companies.
func reminderCompany(doc models.Document) string {
	if doc.Company != nil {
		if name := strings.TrimSpace(doc.Company.Name); name != "" {
			return name
		}
		if legal := strings.TrimSpace(doc.Company.LegalName); legal != "" {
			return legal
		}
	}
	for _, company := range doc.Companies {
		if name := strings.TrimSpace(company.Name); name != "" {
			return name
		}
		if legal := strings.TrimSpace(company.LegalName); legal != "" {
			return legal
		}
	}
	return ""
}

// parseFlexibleDate parses the common date layouts found in extracted KeyFacts.
func parseFlexibleDate(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"02.01.2006",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
	}
	var lastErr error
	for _, layout := range layouts {
		if t, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return t.UTC(), nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}
