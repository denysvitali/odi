package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// buildSearchQuery keeps result searches and facet counts scoped to the same
// text and structured filters. Whitespace-only text still permits filter-only
// searches; nonempty text retains OpenSearch query-string syntax unchanged.
func buildSearchQuery(req SearchRequest) map[string]any {
	query := map[string]any{
		"query_string": map[string]any{
			"query":            req.SearchTerm,
			"fields":           []string{"text", "company.name", "title"},
			"default_operator": "AND",
		},
	}
	if strings.TrimSpace(req.SearchTerm) == "" {
		query = map[string]any{"match_all": map[string]any{}}
	}

	if filters := buildSearchFilters(req); len(filters) > 0 {
		return map[string]any{
			"bool": map[string]any{
				"must":   []any{query},
				"filter": filters,
			},
		}
	}
	return query
}

// streamSearch owns the common transport and response-body lifecycle for raw
// search responses. Callers retain their query, pagination and aggregation
// semantics. operation must be a static label: upstream errors and query bodies
// are deliberately excluded from logs because they may contain document data.
func (s *Server) streamSearch(c *gin.Context, content map[string]any, params opensearchapi.SearchParams, operation string) {
	body, err := json.Marshal(content)
	if err != nil {
		log.Errorf("unable to marshal %s search body", operation)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	resp, err := s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(body),
		Params:  params,
	})
	// The API client may return both a response and an error for upstream
	// failures. Release that response as well as successful search bodies.
	if resp != nil && resp.Inspect().Response != nil {
		defer resp.Inspect().Response.Body.Close()
	}
	if err != nil {
		log.Errorf("unable to perform %s search", operation)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	response := resp.Inspect().Response
	if response.StatusCode >= http.StatusBadRequest {
		log.Errorf("%s search returned status %d", operation, response.StatusCode)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	s.streamResponseBody(c, response.Body, "unable to stream "+operation+" response")
}
