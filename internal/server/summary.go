package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/denysvitali/odi/pkg/models"
)

// summaryResponse is the JSON returned by handleDocumentSummary.
type summaryResponse struct {
	Summary  string           `json:"summary"`
	KeyFacts []models.KeyFact `json:"keyFacts"`
}

// handleDocumentSummary returns an AI-generated TL;DR and key facts for a
// document. The result is cached on the document itself: if the stored
// document already carries a summary we return it without invoking the LLM,
// otherwise we summarize the OCR text, persist the result back to OpenSearch
// (mirroring the lazy thumbnail generate-then-store pattern) and return it.
func (s *Server) handleDocumentSummary(c *gin.Context) {
	docId := c.Param("id")
	if docId == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}
	if !docIdRegexp.MatchString(docId) {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	ctx := c.Request.Context()

	docResp, err := s.osClient.Document.Get(ctx, opensearchapi.DocumentGetReq{
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

	var doc models.Document
	if err := json.Unmarshal(docResp.Source, &doc); err != nil {
		log.Errorf("unable to decode document %s: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	// Cache hit: the summary is already stored on the document.
	if doc.Summary != "" {
		c.JSON(http.StatusOK, summaryResponse{
			Summary:  doc.Summary,
			KeyFacts: doc.KeyFacts,
		})
		return
	}

	// No OCR text to summarize: return an empty (but successful) result so the
	// frontend can show "nothing to summarize" rather than an error.
	if doc.Text == "" {
		c.JSON(http.StatusOK, summaryResponse{
			Summary:  "",
			KeyFacts: []models.KeyFact{},
		})
		return
	}

	client, err := getChatLLMClient()
	if err != nil {
		log.Errorf("unable to build LLM client for summary: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "summary not configured"})
		return
	}
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "summary not configured"})
		return
	}

	result, err := client.Summarize(ctx, doc.Text)
	if err != nil {
		log.Errorf("unable to summarize document %s: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	keyFacts := make([]models.KeyFact, 0, len(result.KeyFacts))
	for _, f := range result.KeyFacts {
		keyFacts = append(keyFacts, models.KeyFact{Label: f.Label, Value: f.Value})
	}

	// Lazily persist the generated summary back onto the document so future
	// requests hit the cache branch above.
	if result.Text != "" {
		s.persistSummary(ctx, docId, result.Text, keyFacts)
	}

	c.JSON(http.StatusOK, summaryResponse{
		Summary:  result.Text,
		KeyFacts: keyFacts,
	})
}

// persistSummary stores the generated summary and key facts back on the
// document via a partial update. Failures are logged but not fatal: the caller
// still returns the freshly computed result to the client.
func (s *Server) persistSummary(ctx context.Context, docId string, summary string, keyFacts []models.KeyFact) {
	updateBody, marshalErr := json.Marshal(map[string]any{
		"doc": map[string]any{
			"summary":  summary,
			"keyFacts": keyFacts,
		},
	})
	if marshalErr != nil {
		log.Warnf("unable to marshal summary update for %s: %v", docId, marshalErr)
		return
	}

	updateResp, err := s.osClient.Update(ctx, opensearchapi.UpdateReq{
		Index:      s.osIndex,
		DocumentID: docId,
		Body:       bytes.NewReader(updateBody),
		Params:     opensearchapi.UpdateParams{Refresh: "true"},
	})
	if err != nil {
		log.Warnf("unable to persist summary for %s: %v", docId, err)
		return
	}
	defer updateResp.Inspect().Response.Body.Close()

	if updateResp.Inspect().Response.StatusCode >= 400 {
		log.Warnf("persist summary for %s returned %s", docId, updateResp.Inspect().Response.Status())
	}
}
