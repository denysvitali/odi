package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/denysvitali/odi/pkg/storage/model"
	"github.com/denysvitali/odi/pkg/thumbnailer"
)

type SearchRequest struct {
	SearchTerm string   `json:"searchTerm"`
	Size       int      `json:"size,omitempty" binding:"omitempty,max=1000"`
	ScrollId   string   `json:"scrollId,omitempty"`
	Companies  []string `json:"companies,omitempty"`
	DateFrom   string   `json:"dateFrom,omitempty"`
	DateTo     string   `json:"dateTo,omitempty"`
	HasBarcode *bool    `json:"hasBarcode,omitempty"`
	Title      string   `json:"title,omitempty"`
	DocTypes   []string `json:"docTypes,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func (s *Server) handleSearch(c *gin.Context) {
	var searchRequest SearchRequest
	err := c.BindJSON(&searchRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	if searchRequest.ScrollId != "" {
		s.streamScroll(c, searchRequest.ScrollId, fmt.Sprintf("scrollId=%q", searchRequest.ScrollId))
		return
	}

	// Initial search with scroll enabled
	size := searchRequest.Size
	if size <= 0 {
		size = 50
	}
	if size > maxSearchSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("size exceeds maximum of %d", maxSearchSize)})
		return
	}

	queryString := map[string]any{
		"query_string": map[string]any{
			"query":            searchRequest.SearchTerm,
			"fields":           []string{"text", "company.name", "title"},
			"default_operator": "AND",
		},
	}

	filters := buildSearchFilters(searchRequest)

	query := queryString
	if len(filters) > 0 {
		query = map[string]any{
			"bool": map[string]any{
				"must":   []any{queryString},
				"filter": filters,
			},
		}
	}

	searchContent := map[string]any{
		"size":  size,
		"query": query,
		"highlight": map[string]any{
			"fields": map[string]any{
				"text": map[string]any{},
			},
		},
	}

	jsonBody, marshalErr := json.Marshal(searchContent)
	if marshalErr != nil {
		log.Errorf("unable to marshal search body for term=%q size=%d: %v", searchRequest.SearchTerm, size, marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	searchResp, err := s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
		Params: opensearchapi.SearchParams{
			Scroll: 10 * time.Minute,
		},
	})
	if err != nil {
		log.Errorf("unable to perform search (term=%q size=%d): %v", searchRequest.SearchTerm, size, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer searchResp.Inspect().Response.Body.Close()

	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("search returned error (term=%q): %s", searchRequest.SearchTerm, searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	s.streamResponseBody(c, searchResp.Inspect().Response.Body, fmt.Sprintf("unable to stream search response (term=%q)", searchRequest.SearchTerm))
}

// buildSearchFilters constructs OpenSearch filter clauses from the structured
// filter fields in the search request. Returns nil when no filters are set.
func buildSearchFilters(req SearchRequest) []map[string]any {
	var filters []map[string]any

	if len(req.Companies) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string]any{
				"company.name.keyword": req.Companies,
			},
		})
	}

	if req.DateFrom != "" || req.DateTo != "" {
		rangeFilter := map[string]any{}
		if req.DateFrom != "" {
			rangeFilter["gte"] = req.DateFrom
		}
		if req.DateTo != "" {
			rangeFilter["lte"] = req.DateTo
		}
		filters = append(filters, map[string]any{
			"range": map[string]any{
				"date": rangeFilter,
			},
		})
	}

	if req.HasBarcode != nil {
		filters = append(filters, map[string]any{
			"exists": map[string]any{
				"field": "barcode",
			},
		})
	}

	if req.Title != "" {
		filters = append(filters, map[string]any{
			"match": map[string]any{
				"title": req.Title,
			},
		})
	}

	filters = append(filters, buildDocTypeTagFilters(req.DocTypes, req.Tags)...)

	return filters
}

// SearchFacetsRequest carries the same filter fields as SearchRequest but no
// pagination or scroll controls — it is used to request aggregation buckets.
type SearchFacetsRequest struct {
	SearchTerm string   `json:"searchTerm"`
	Companies  []string `json:"companies,omitempty"`
	DateFrom   string   `json:"dateFrom,omitempty"`
	DateTo     string   `json:"dateTo,omitempty"`
	HasBarcode *bool    `json:"hasBarcode,omitempty"`
	Title      string   `json:"title,omitempty"`
	DocTypes   []string `json:"docTypes,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

func (s *Server) handleSearchFacets(c *gin.Context) {
	var req SearchFacetsRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	queryString := map[string]any{
		"query_string": map[string]any{
			"query":            req.SearchTerm,
			"fields":           []string{"text", "company.name", "title"},
			"default_operator": "AND",
		},
	}

	filters := buildSearchFilters(SearchRequest{
		Companies:  req.Companies,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		HasBarcode: req.HasBarcode,
		Title:      req.Title,
		DocTypes:   req.DocTypes,
		Tags:       req.Tags,
	})

	query := queryString
	if len(filters) > 0 {
		query = map[string]any{
			"bool": map[string]any{
				"must":   []any{queryString},
				"filter": filters,
			},
		}
	}

	aggs := map[string]any{
		"companies": map[string]any{
			"terms": map[string]any{
				"field": "company.name.keyword",
				"size":  20,
			},
		},
		"date_histogram": map[string]any{
			"date_histogram": map[string]any{
				"field":             "date",
				"calendar_interval": "month",
			},
		},
		"barcode_count": map[string]any{
			"filter": map[string]any{
				"exists": map[string]any{
					"field": "barcode",
				},
			},
		},
	}
	for k, v := range docTypeTagAggs() {
		aggs[k] = v
	}

	searchContent := map[string]any{
		"size":  0,
		"query": query,
		"aggs":  aggs,
	}

	jsonBody, marshalErr := json.Marshal(searchContent)
	if marshalErr != nil {
		log.Errorf("unable to marshal facets body for term=%q: %v", req.SearchTerm, marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	searchResp, err := s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
	})
	if err != nil {
		log.Errorf("unable to perform facets search (term=%q): %v", req.SearchTerm, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer searchResp.Inspect().Response.Body.Close()

	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("facets search returned error (term=%q): %s", req.SearchTerm, searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	s.streamResponseBody(c, searchResp.Inspect().Response.Body, fmt.Sprintf("unable to stream facets response (term=%q)", req.SearchTerm))
}

// streamScroll continues an OpenSearch scroll request and streams the raw
// response body back to the client. logCtx annotates any error logs.
func (s *Server) streamScroll(c *gin.Context, scrollID, logCtx string) {
	// Extend the TTL on every page so paging stays alive across many loads.
	scrollResp, err := s.osClient.Scroll.Get(c.Request.Context(), opensearchapi.ScrollGetReq{
		ScrollID: scrollID,
		Params: opensearchapi.ScrollGetParams{
			Scroll: 10 * time.Minute,
		},
	})
	if err != nil {
		log.Errorf("unable to perform scroll (%s): %v", logCtx, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer scrollResp.Inspect().Response.Body.Close()
	if scrollResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("scroll returned error (%s): %s", logCtx, scrollResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	s.streamResponseBody(c, scrollResp.Inspect().Response.Body, fmt.Sprintf("unable to stream scroll response (%s)", logCtx))
}

// streamResponseBody copies a raw JSON response body to the client, logging a
// streaming error with errMsg if the copy fails mid-flight.
func (s *Server) streamResponseBody(c *gin.Context, body io.Reader, errMsg string) {
	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/json")
	if _, err := io.Copy(c.Writer, body); err != nil {
		logStreamError(c, err, errMsg)
	}
}

func (s *Server) returnDocument(c *gin.Context, scanID string, sequenceIdStr string) {
	sequenceId, err := strconv.ParseInt(sequenceIdStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	page, err := s.storage.Retrieve(c.Request.Context(), scanID, int(sequenceId))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, model.ErrNotFound) {
			log.Debugf("page not found: scan=%s seq=%d", scanID, sequenceId)
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("page not found: scan=%s seq=%d", scanID, sequenceId),
			})
			return
		}
		log.Errorf("unable to retrieve page scan=%s seq=%d: %v", scanID, sequenceId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	// Detect content type from the first 512 bytes, then seek back.
	buf := make([]byte, 512)
	n, _ := page.Reader.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if rs, ok := page.Reader.(io.Seeker); ok {
		_, _ = rs.Seek(0, io.SeekStart)
	}

	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	_, err = io.Copy(c.Writer, page.Reader)
	if err != nil {
		logStreamError(c, err, fmt.Sprintf("unable to stream page scan=%s seq=%d", scanID, sequenceId))
		return
	}
}

func (s *Server) handleGetFile(c *gin.Context) {
	scanID := c.Param("scanID")
	sequenceIdStr := c.Param("sequenceId")

	if scanID == "" || sequenceIdStr == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	s.returnDocument(c, scanID, sequenceIdStr)
}

func (s *Server) handleGetThumbnail(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	matches := docIdRegexp.FindStringSubmatch(id)
	if len(matches) != 3 {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	scanID := matches[1]
	sequenceIdStr := matches[2]
	sequenceId, err := strconv.ParseInt(sequenceIdStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	// Try thumbnail storage first
	if ts, ok := s.storage.(model.ThumbnailStorage); ok {
		ctx := c.Request.Context()

		exists, err := ts.ThumbnailExists(ctx, scanID, int(sequenceId))
		if err != nil {
			log.Warnf("error checking thumbnail existence: %v", err)
		} else if exists {
			thumb, err := ts.RetrieveThumbnail(ctx, scanID, int(sequenceId))
			if err != nil {
				log.Warnf("error retrieving thumbnail: %v", err)
			} else {
				c.Header("Content-Type", "image/jpeg")
				c.Status(http.StatusOK)
				_, err = io.Copy(c.Writer, thumb.Reader)
				if err != nil {
					logStreamError(c, err, "unable to stream thumbnail")
				}
				return
			}
		}

		// Thumbnail doesn't exist, generate it
		log.Debugf("generating thumbnail for %s_%d", scanID, sequenceId)
		page, err := s.storage.Retrieve(ctx, scanID, int(sequenceId))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, model.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
				return
			}
			log.Errorf("error retrieving original page: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}

		// Generate thumbnail
		_, err = page.Reader.Seek(0, io.SeekStart)
		if err != nil {
			log.Errorf("error seeking page reader: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}

		thumbReader, err := thumbnailer.Generate(page.Reader)
		if err != nil {
			log.Errorf("error generating thumbnail: %v", err)
			// Fall back to original
			s.returnDocument(c, scanID, sequenceIdStr)
			return
		}

		// Store thumbnail for future requests
		err = ts.StoreThumbnail(ctx, scanID, int(sequenceId), thumbReader)
		if err != nil {
			log.Warnf("error storing thumbnail: %v", err)
			// Continue anyway - we still have the thumbnail to return
		}

		// Return thumbnail
		c.Header("Content-Type", "image/jpeg")
		c.Status(http.StatusOK)
		if rs, ok := thumbReader.(io.ReadSeeker); ok {
			_, err = rs.Seek(0, io.SeekStart)
			if err != nil {
				log.Errorf("error seeking generated thumbnail: %v", err)
				return
			}
		}
		_, err = io.Copy(c.Writer, thumbReader)
		if err != nil {
			logStreamError(c, err, "unable to stream generated thumbnail")
		}
		return
	}

	// Thumbnail storage not available, fall back to original
	log.Warn("storage does not implement ThumbnailStorage, returning original image")
	s.returnDocument(c, scanID, sequenceIdStr)
}

var badRequest = gin.H{
	"error": "bad request",
}

var internalServerError = gin.H{
	"error": "internal server error",
}

type Document[T any] struct {
	Index       string `json:"_index"`
	Id          string `json:"_id"`
	Version     int    `json:"_version"`
	SeqNo       int    `json:"_seq_no"`
	PrimaryTerm int    `json:"_primary_term"`
	Found       bool   `json:"found"`
	Source      T      `json:"_source"`
}

var docIdRegexp = regexp.MustCompile("^([0-9a-f-]+)_([0-9]+)$")

func (s *Server) handleGetDocument(c *gin.Context) {
	docId := c.Param("id")
	if docId == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	if !docIdRegexp.MatchString(docId) {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	docResp, err := s.osClient.Document.Get(c.Request.Context(), opensearchapi.DocumentGetReq{
		Index:      s.osIndex,
		DocumentID: docId,
	})
	if err != nil {
		log.Errorf("unable to fetch document %s from OpenSearch: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	statusCode := docResp.Inspect().Response.StatusCode
	if statusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	if statusCode >= 400 {
		log.Warnf("unable to get document %s: %s", docId, docResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.JSON(http.StatusOK, docResp.Source)
}

func (s *Server) handleGetDocuments(c *gin.Context) {
	scrollId := c.Query("scroll_id")
	var searchResp *opensearchapi.SearchResp
	var err error

	if scrollId != "" {
		s.streamScroll(c, scrollId, "documents scroll")
		return
	}

	// Initial search with scroll
	size := 50
	if sizeStr := c.Query("size"); sizeStr != "" {
		if parsed, parseErr := strconv.Atoi(sizeStr); parseErr == nil && parsed > 0 {
			size = parsed
		}
	}
	if size > maxSearchSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("size exceeds maximum of %d", maxSearchSize)})
		return
	}

	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		t, err := time.Parse("2006-01-02", dateFromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid date_from: %v", err)})
			return
		}
		dateFrom = &t
	}
	if dateToStr := c.Query("date_to"); dateToStr != "" {
		t, err := time.Parse("2006-01-02", dateToStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid date_to: %v", err)})
			return
		}
		endOfDay := t.Add(24*time.Hour - time.Second)
		dateTo = &endOfDay
	}

	hasDateFilter := dateFrom != nil || dateTo != nil

	searchBody := map[string]any{
		"size": size,
		"sort": []map[string]any{{"indexedAt": "desc"}},
	}

	if hasDateFilter {
		var filters []map[string]any
		if dateFrom != nil {
			filters = append(filters, map[string]any{
				"range": map[string]any{"date": map[string]any{"gte": dateFrom.Format(time.RFC3339)}},
			})
		}
		if dateTo != nil {
			filters = append(filters, map[string]any{
				"range": map[string]any{"date": map[string]any{"lte": dateTo.Format(time.RFC3339)}},
			})
		}
		searchBody["query"] = map[string]any{
			"bool": map[string]any{"filter": filters},
		}
	}
	jsonBody, marshalErr := json.Marshal(searchBody)
	if marshalErr != nil {
		log.Errorf("unable to marshal search body: %v", marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	searchResp, err = s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
		Params: opensearchapi.SearchParams{
			Scroll: 10 * time.Minute,
		},
	})
	if err != nil {
		log.Errorf("unable to get documents: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer searchResp.Inspect().Response.Body.Close()

	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Warnf("unable to get documents: %s", searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	s.streamResponseBody(c, searchResp.Inspect().Response.Body, "unable to stream documents response")
}
