package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/denysvitali/odi/pkg/models"
)

// insightsScrollTTL keeps each scroll page alive long enough to walk the whole
// result set, mirroring the 10 * time.Minute window used by handleGetDocuments.
const insightsScrollTTL = 10 * time.Minute

// insightsScrollSize is the per-page hit count for the scroll over documents
// that carry key facts.
const insightsScrollSize = 500

// amountLabelRegexp matches the KeyFact labels that describe a monetary value
// we want to aggregate (amount / total / due / paid). It is anchored loosely so
// labels like "Amount due" or "Total paid" all match. Same regexp style as
// docIdRegexp in handlers.go.
var amountLabelRegexp = regexp.MustCompile(`(?i)\b(amount|total|due|paid|balance|sum)\b`)

// amountValueRegexp extracts an optional 3-letter currency code and a decimal
// number from a KeyFact value such as "CHF 1,240.00" or "EUR 99.90". The number
// may use thousands separators (',' or '.' or ' ') which we normalize before
// parsing.
var amountValueRegexp = regexp.MustCompile(`(?i)([A-Z]{3})?\s*([0-9][0-9.,' ]*[0-9]|[0-9])`)

// InsightBucket is a single aggregation row: a label (company / year / docType)
// with the summed amount and the number of contributing documents.
type InsightBucket struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
	Count int     `json:"count"`
}

// InsightsResponse is the JSON returned by handleInsights.
type InsightsResponse struct {
	ByCompany []InsightBucket `json:"byCompany"`
	ByYear    []InsightBucket `json:"byYear"`
	ByDocType []InsightBucket `json:"byDocType"`
	Currency  string          `json:"currency"`
	Total     float64         `json:"total"`
	Count     int             `json:"count"`
}

// insightsHit is the minimal projection of an OpenSearch hit needed to build
// the spend buckets.
type insightsHit struct {
	Source models.Document `json:"_source"`
}

type insightsScrollResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []insightsHit `json:"hits"`
	} `json:"hits"`
}

// parseAmount normalizes a KeyFact value into a float and the detected currency
// code (uppercased, empty when none was found). The boolean reports whether a
// usable amount could be parsed.
func parseAmount(value string) (float64, string, bool) {
	matches := amountValueRegexp.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return 0, "", false
	}

	currency := strings.ToUpper(matches[1])
	raw := matches[2]

	// Strip spaces and apostrophes used as thousands separators (Swiss style).
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "'", "")

	// Decide which separator is the decimal point. If both ',' and '.' appear,
	// the right-most one is the decimal separator and the other is a thousands
	// separator. If only ',' appears, treat it as the decimal separator.
	lastComma := strings.LastIndex(raw, ",")
	lastDot := strings.LastIndex(raw, ".")
	switch {
	case lastComma >= 0 && lastDot >= 0:
		if lastComma > lastDot {
			raw = strings.ReplaceAll(raw, ".", "")
			raw = strings.Replace(raw, ",", ".", 1)
		} else {
			raw = strings.ReplaceAll(raw, ",", "")
		}
	case lastComma >= 0:
		raw = strings.Replace(raw, ",", ".", 1)
	}

	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, currency, false
	}
	return amount, currency, true
}

// documentAmount returns the first parseable monetary KeyFact on a document
// (label matching amountLabelRegexp), the detected currency, and whether a
// match was found.
func documentAmount(doc models.Document) (float64, string, bool) {
	for _, fact := range doc.KeyFacts {
		if !amountLabelRegexp.MatchString(fact.Label) {
			continue
		}
		amount, currency, ok := parseAmount(fact.Value)
		if ok {
			return amount, currency, true
		}
	}
	return 0, "", false
}

// documentCompany returns a display name for the document's company, falling
// back to the first entry in Companies, then to "Unknown".
func documentCompany(doc models.Document) string {
	if doc.Company != nil && strings.TrimSpace(doc.Company.Name) != "" {
		return doc.Company.Name
	}
	for _, c := range doc.Companies {
		if strings.TrimSpace(c.Name) != "" {
			return c.Name
		}
	}
	return "Unknown"
}

// documentYear returns the 4-digit year string for the document's date, or
// "Unknown" when it has none.
func documentYear(doc models.Document) string {
	if doc.Date == nil || doc.Date.IsZero() {
		return "Unknown"
	}
	return strconv.Itoa(doc.Date.Year())
}

// insightsAccumulator buckets parsed amounts by company, year and doc type.
type insightsAccumulator struct {
	byCompany map[string]*InsightBucket
	byYear    map[string]*InsightBucket
	byDocType map[string]*InsightBucket
	currency  string
	total     float64
	count     int
}

func newInsightsAccumulator() *insightsAccumulator {
	return &insightsAccumulator{
		byCompany: map[string]*InsightBucket{},
		byYear:    map[string]*InsightBucket{},
		byDocType: map[string]*InsightBucket{},
	}
}

func addToBucket(buckets map[string]*InsightBucket, name string, amount float64) {
	b, ok := buckets[name]
	if !ok {
		b = &InsightBucket{Name: name}
		buckets[name] = b
	}
	b.Total += amount
	b.Count++
}

// add records a single document's amount into every bucket dimension. yearFilter
// (when non-empty) restricts which documents are counted.
func (a *insightsAccumulator) add(doc models.Document, yearFilter string) {
	amount, currency, ok := documentAmount(doc)
	if !ok {
		return
	}

	year := documentYear(doc)
	if yearFilter != "" && year != yearFilter {
		return
	}

	if a.currency == "" && currency != "" {
		a.currency = currency
	}

	docType := doc.DocType
	if strings.TrimSpace(docType) == "" {
		docType = "Unknown"
	}

	addToBucket(a.byCompany, documentCompany(doc), amount)
	addToBucket(a.byYear, year, amount)
	addToBucket(a.byDocType, docType, amount)
	a.total += amount
	a.count++
}

// sortedBuckets flattens a bucket map into a slice ordered by descending total
// (ties broken alphabetically) for stable, deterministic output.
func sortedBuckets(buckets map[string]*InsightBucket) []InsightBucket {
	out := make([]InsightBucket, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// byYearChronological flattens the year buckets sorted ascending by year so the
// dashboard can render a left-to-right timeline.
func byYearChronological(buckets map[string]*InsightBucket) []InsightBucket {
	out := make([]InsightBucket, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (a *insightsAccumulator) response() InsightsResponse {
	currency := a.currency
	if currency == "" {
		currency = "CHF"
	}
	return InsightsResponse{
		ByCompany: sortedBuckets(a.byCompany),
		ByYear:    byYearChronological(a.byYear),
		ByDocType: sortedBuckets(a.byDocType),
		Currency:  currency,
		Total:     a.total,
		Count:     a.count,
	}
}

// effectiveDateRange folds the optional ?year= filter into the explicit
// date_from/date_to bounds and returns the intersection. A year N maps to the
// closed interval [N-01-01, N-12-31]; when both a year and explicit bounds are
// given the effective range is their overlap. The returned strings are the
// gte/lte bounds (empty when unbounded on that side). The bool reports whether
// the resulting range is non-empty; an impossible range (gte > lte) yields
// ok=false so the caller can short-circuit to an empty result.
func effectiveDateRange(yearFilter, dateFrom, dateTo string) (gte, lte string, ok bool) {
	gte, lte = dateFrom, dateTo
	if yearFilter != "" {
		yearFrom := yearFilter + "-01-01"
		yearTo := yearFilter + "-12-31"
		// Intersect: take the later lower bound and the earlier upper bound.
		// Lexicographic comparison is correct for ISO-8601 date strings.
		if gte == "" || yearFrom > gte {
			gte = yearFrom
		}
		if lte == "" || yearTo < lte {
			lte = yearTo
		}
	}
	if gte != "" && lte != "" && gte > lte {
		return "", "", false
	}
	return gte, lte, true
}

// collectInsights walks every document that carries key facts via an OpenSearch
// scroll, parses amounts server-side and aggregates them. yearFilter, dateFrom
// and dateTo (all optional) narrow the scanned set. The year filter is pushed
// down into the OpenSearch query as a date range (intersected with any explicit
// date_from/date_to) so a year-scoped request does not scroll the whole corpus.
func (s *Server) collectInsights(c *gin.Context, yearFilter, dateFrom, dateTo string) (*insightsAccumulator, error) {
	ctx := c.Request.Context()

	gte, lte, rangeOk := effectiveDateRange(yearFilter, dateFrom, dateTo)
	if !rangeOk {
		// An empty intersection (e.g. year=2024 with date_to=2023-01-01) can
		// never match a document; return an empty accumulator without querying.
		return newInsightsAccumulator(), nil
	}

	filters := []map[string]any{
		{"exists": map[string]any{"field": "keyFacts"}},
	}
	if gte != "" || lte != "" {
		rangeFilter := map[string]any{}
		if gte != "" {
			rangeFilter["gte"] = gte
		}
		if lte != "" {
			rangeFilter["lte"] = lte
		}
		filters = append(filters, map[string]any{
			"range": map[string]any{"date": rangeFilter},
		})
	}

	searchBody := map[string]any{
		"size": insightsScrollSize,
		"query": map[string]any{
			"bool": map[string]any{"filter": filters},
		},
		"_source": []string{"company", "companies", "date", "docType", "keyFacts"},
	}

	jsonBody, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("marshal insights query: %w", err)
	}

	searchResp, err := s.osClient.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
		Params: opensearchapi.SearchParams{
			Scroll: insightsScrollTTL,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("perform insights search: %w", err)
	}

	acc := newInsightsAccumulator()
	scrollID, _, err := drainInsightsPage(searchResp.Inspect().Response.Body, acc, yearFilter)
	if err != nil {
		return nil, err
	}

	// Continue scrolling until a page returns no further hits.
	for scrollID != "" {
		scrollResp, scrollErr := s.osClient.Scroll.Get(ctx, opensearchapi.ScrollGetReq{
			ScrollID: scrollID,
			Params:   opensearchapi.ScrollGetParams{Scroll: insightsScrollTTL},
		})
		if scrollErr != nil {
			return nil, fmt.Errorf("scroll insights: %w", scrollErr)
		}

		nextID, hits, drainErr := drainInsightsPage(scrollResp.Inspect().Response.Body, acc, yearFilter)
		if drainErr != nil {
			return nil, drainErr
		}
		scrollID = nextID
		if hits == 0 {
			break
		}
	}

	// Best-effort cleanup of the scroll context.
	if scrollID != "" {
		clearResp, clearErr := s.osClient.Scroll.Delete(ctx, opensearchapi.ScrollDeleteReq{
			ScrollIDs: []string{scrollID},
		})
		if clearErr != nil {
			log.Warnf("unable to clear insights scroll: %v", clearErr)
		} else {
			clearResp.Inspect().Response.Body.Close()
		}
	}

	return acc, nil
}

// drainInsightsPage decodes one scroll page into the accumulator, closing the
// body, and reports the next scroll ID plus the number of hits on this page.
func drainInsightsPage(body io.ReadCloser, acc *insightsAccumulator, yearFilter string) (string, int, error) {
	defer body.Close()

	var page insightsScrollResponse
	if err := json.NewDecoder(body).Decode(&page); err != nil {
		return "", 0, fmt.Errorf("decode insights page: %w", err)
	}

	for _, hit := range page.Hits.Hits {
		acc.add(hit.Source, yearFilter)
	}

	return page.ScrollID, len(page.Hits.Hits), nil
}

// handleInsights aggregates the AI-extracted amounts on indexed documents into
// a spend dashboard bucketed by company, year and document type. Optional
// ?year=, ?date_from= and ?date_to= query parameters narrow the result set.
func (s *Server) handleInsights(c *gin.Context) {
	yearFilter, dateFrom, dateTo, ok := parseInsightsParams(c)
	if !ok {
		return
	}

	acc, err := s.collectInsights(c, yearFilter, dateFrom, dateTo)
	if err != nil {
		log.Errorf("unable to collect insights: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.JSON(http.StatusOK, acc.response())
}

// handleInsightsCSV emits the per-company and per-year spend rows as a CSV
// attachment.
func (s *Server) handleInsightsCSV(c *gin.Context) {
	yearFilter, dateFrom, dateTo, ok := parseInsightsParams(c)
	if !ok {
		return
	}

	acc, err := s.collectInsights(c, yearFilter, dateFrom, dateTo)
	if err != nil {
		log.Errorf("unable to collect insights for csv: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	resp := acc.response()

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="insights.csv"`)
	c.Status(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"dimension", "name", "total", "count", "currency"})
	for _, b := range resp.ByCompany {
		_ = w.Write([]string{"company", b.Name, strconv.FormatFloat(b.Total, 'f', 2, 64), strconv.Itoa(b.Count), resp.Currency})
	}
	for _, b := range resp.ByYear {
		_ = w.Write([]string{"year", b.Name, strconv.FormatFloat(b.Total, 'f', 2, 64), strconv.Itoa(b.Count), resp.Currency})
	}
	for _, b := range resp.ByDocType {
		_ = w.Write([]string{"docType", b.Name, strconv.FormatFloat(b.Total, 'f', 2, 64), strconv.Itoa(b.Count), resp.Currency})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		logStreamError(c, err, "unable to stream insights csv")
	}
}

// parseInsightsParams validates and extracts the shared query parameters used
// by both insights handlers. It writes a 400 and returns ok=false on bad input.
func parseInsightsParams(c *gin.Context) (yearFilter, dateFrom, dateTo string, ok bool) {
	yearFilter = strings.TrimSpace(c.Query("year"))
	if yearFilter != "" {
		if _, err := strconv.Atoi(yearFilter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return "", "", "", false
		}
	}

	if v := strings.TrimSpace(c.Query("date_from")); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid date_from: %v", err)})
			return "", "", "", false
		}
		dateFrom = v
	}
	if v := strings.TrimSpace(c.Query("date_to")); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid date_to: %v", err)})
			return "", "", "", false
		}
		dateTo = v
	}

	return yearFilter, dateFrom, dateTo, true
}
