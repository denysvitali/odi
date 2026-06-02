package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// errSimilarSearchFailed is returned when the more_like_this query responds
// with an error status.
var errSimilarSearchFailed = errors.New("similar documents search failed")

// similarSize is the number of related documents returned by the
// More-Like-This rail.
const similarSize = 8

// similarDocument is the minimal projection of a related document returned to
// the frontend rail. It mirrors the fields rendered by SimilarDocuments.vue.
type similarDocument struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Date    string `json:"date,omitempty"`
	DocType string `json:"docType,omitempty"`
	Company string `json:"company,omitempty"`
}

// similarResponse is the JSON envelope returned by handleSimilarDocuments.
type similarResponse struct {
	Documents []similarDocument `json:"documents"`
}

// similarHit is the OpenSearch hit shape we decode for the rail. We pull the
// scanID so we can attribute hits and the metadata fields we project back.
type similarHit struct {
	ID     string `json:"_id"`
	Source struct {
		Title   string         `json:"title"`
		Date    *string        `json:"date"`
		DocType string         `json:"docType"`
		ScanID  string         `json:"scanID"`
		Company *companySource `json:"company"`
	} `json:"_source"`
}

// companySource decodes the nested company object so we can surface its name
// on each rail card.
type companySource struct {
	Name string `json:"name"`
}

type similarSearchResponse struct {
	Hits struct {
		Hits []similarHit `json:"hits"`
	} `json:"hits"`
}

// handleSimilarDocuments surfaces the most textually similar scans to the one
// identified by :id. It fetches the source document to obtain its _index/_id,
// then issues an OpenSearch more_like_this query over the OCR text, title and
// company name. Same-document and same-scan hits are excluded so the rail shows
// genuinely distinct neighbours. This is pure OpenSearch (no LLM) so it works
// in headless deployments.
func (s *Server) handleSimilarDocuments(c *gin.Context) {
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

	// We only need the document to (a) confirm it exists and (b) read its
	// scanID so we can exclude same-scan siblings from the rail. We deliberately
	// restrict _source to scanID via SourceIncludes: the full source includes the
	// OCR `text` field which can be megabytes, and transferring/decoding it here
	// is pure waste — the more_like_this query references the source doc by
	// {_index,_id}, so OpenSearch uses its analysed terms internally and never
	// needs the raw text round-tripped through this handler.
	docResp, err := s.osClient.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index:      s.osIndex,
		DocumentID: docId,
		Params: opensearchapi.DocumentGetParams{
			SourceIncludes: []string{"scanID"},
		},
	})
	// The OpenSearch client returns a non-nil error for any non-2xx status, but
	// still hands back the response, so we inspect the status code first to
	// distinguish a genuine "not found" from a transport/server failure.
	if docResp != nil {
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
	}
	if err != nil {
		log.Errorf("unable to fetch document %s from OpenSearch: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	// Only scanID is projected (see SourceIncludes above), so decode into a
	// minimal struct rather than the full models.Document.
	var doc struct {
		ScanID string `json:"scanID"`
	}
	if err := json.Unmarshal(docResp.Source, &doc); err != nil {
		log.Errorf("unable to decode document %s: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	// Note: we no longer short-circuit on an empty OCR `text` field. Checking it
	// would require fetching the (potentially huge) text we just went out of our
	// way to avoid transferring. The early return was only an optimisation, not a
	// correctness requirement: similarHits already excludes the source document
	// itself, so a text-less seed simply yields a more_like_this result with
	// few/no hits, which is the same empty rail the caller would have seen.
	hits, err := s.similarHits(ctx, docResp.Index, docId, doc.ScanID)
	if err != nil {
		log.Errorf("unable to gather similar documents for %s: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	documents := make([]similarDocument, 0, len(hits))
	for _, hit := range hits {
		sd := similarDocument{
			ID:      hit.ID,
			Title:   hit.Source.Title,
			DocType: hit.Source.DocType,
		}
		if hit.Source.Date != nil {
			sd.Date = *hit.Source.Date
		}
		if hit.Source.Company != nil {
			sd.Company = hit.Source.Company.Name
		}
		documents = append(documents, sd)
	}

	c.JSON(http.StatusOK, similarResponse{Documents: documents})
}

// similarHits runs the more_like_this query against OpenSearch and returns the
// decoded hits. The source document is referenced by {_index,_id} so OpenSearch
// uses its analyzed terms as the seed; the source _id and its scanID are
// excluded to drop the document itself and its same-scan siblings.
func (s *Server) similarHits(ctx context.Context, srcIndex, srcID, scanID string) ([]similarHit, error) {
	mustNot := []any{
		map[string]any{
			"ids": map[string]any{
				"values": []string{srcID},
			},
		},
	}
	if scanID != "" {
		mustNot = append(mustNot, map[string]any{
			"term": map[string]any{
				"scanID": scanID,
			},
		})
	}

	searchContent := map[string]any{
		"size": similarSize,
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{
					map[string]any{
						"more_like_this": map[string]any{
							"fields": []string{"text", "title", "company.name"},
							"like": []any{
								map[string]any{
									"_index": srcIndex,
									"_id":    srcID,
								},
							},
							"min_term_freq": 1,
							"min_doc_freq":  1,
						},
					},
				},
				"must_not": mustNot,
			},
		},
		"_source": []string{"title", "date", "docType", "scanID", "company"},
	}

	jsonBody, err := json.Marshal(searchContent)
	if err != nil {
		return nil, err
	}

	searchResp, err := s.osClient.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
	})
	if err != nil {
		return nil, err
	}
	defer searchResp.Inspect().Response.Body.Close()

	if searchResp.Inspect().Response.StatusCode >= 400 {
		return nil, errSimilarSearchFailed
	}

	var parsed similarSearchResponse
	if err := json.NewDecoder(searchResp.Inspect().Response.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	return parsed.Hits.Hits, nil
}
