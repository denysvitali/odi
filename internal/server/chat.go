package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/denysvitali/odi/pkg/llm"
)

// errChatSearchFailed is returned when the OpenSearch grounding query responds
// with an error status.
var errChatSearchFailed = errors.New("chat grounding search failed")

// chatSearchSize is the number of top documents fed to the LLM as grounding
// context for a chat answer.
const chatSearchSize = 6

// ChatRequest carries the user's question plus the same structured filter
// fields as SearchRequest, so chat answers can be scoped the same way as
// search.
type ChatRequest struct {
	Question   string   `json:"question"`
	Companies  []string `json:"companies,omitempty"`
	DateFrom   string   `json:"dateFrom,omitempty"`
	DateTo     string   `json:"dateTo,omitempty"`
	HasBarcode *bool    `json:"hasBarcode,omitempty"`
	Title      string   `json:"title,omitempty"`
}

// chatHit is the minimal projection of an OpenSearch hit needed to build a
// grounding passage.
type chatHit struct {
	ID     string `json:"_id"`
	Source struct {
		Text  string `json:"text"`
		Title string `json:"title"`
	} `json:"_source"`
}

type chatSearchResponse struct {
	Hits struct {
		Hits []chatHit `json:"hits"`
	} `json:"hits"`
}

// llmClientOnce lazily constructs the chat LLM client from the LLM_API_ADDR
// (or LLM_ADDR) environment variable. We cache it here on package-level state
// to avoid editing the shared Server struct in server.go.
var (
	llmClientOnce sync.Once
	llmClient     *llm.Client
	llmClientErr  error
)

// chatLLMAddr resolves the configured LLM base URL, mirroring the env vars
// consumed by the CLI (LLM_API_ADDR), with LLM_ADDR accepted as an alias.
func chatLLMAddr() string {
	if v := strings.TrimSpace(os.Getenv("LLM_API_ADDR")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("LLM_ADDR"))
}

// getChatLLMClient returns a cached LLM client, or nil when no LLM address is
// configured.
func getChatLLMClient() (*llm.Client, error) {
	llmClientOnce.Do(func() {
		addr := chatLLMAddr()
		if addr == "" {
			return
		}
		opts := []llm.Option{}
		if model := strings.TrimSpace(os.Getenv("LLM_MODEL")); model != "" {
			opts = append(opts, llm.WithModel(model))
		}
		llmClient, llmClientErr = llm.New(addr, opts...)
	})
	return llmClient, llmClientErr
}

func (s *Server) handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	if strings.TrimSpace(req.Question) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question is required"})
		return
	}

	client, err := getChatLLMClient()
	if err != nil {
		log.Errorf("unable to build chat LLM client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat not configured"})
		return
	}
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat not configured"})
		return
	}

	passages, err := s.chatPassages(c, req)
	if err != nil {
		log.Errorf("unable to gather chat passages (question=%q): %v", req.Question, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if len(passages) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"answer":    "No relevant documents found.",
			"citations": []string{},
		})
		return
	}

	answer, err := client.Answer(c.Request.Context(), req.Question, passages)
	if err != nil {
		log.Errorf("unable to generate chat answer (question=%q): %v", req.Question, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	citations := make([]string, 0, len(passages))
	for _, p := range passages {
		citations = append(citations, p.DocID)
	}

	c.JSON(http.StatusOK, gin.H{
		"answer":    answer,
		"citations": citations,
	})
}

// chatPassages runs a query_string search over the OCR text and metadata and
// returns the top hits as grounding passages for the LLM.
func (s *Server) chatPassages(c *gin.Context, req ChatRequest) ([]llm.Passage, error) {
	queryString := map[string]any{
		"query_string": map[string]any{
			"query":            req.Question,
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

	searchContent := map[string]any{
		"size":    chatSearchSize,
		"query":   query,
		"_source": []string{"text", "title"},
	}

	jsonBody, err := json.Marshal(searchContent)
	if err != nil {
		return nil, err
	}

	searchResp, err := s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
	})
	if err != nil {
		return nil, err
	}
	defer searchResp.Inspect().Response.Body.Close()

	if searchResp.Inspect().Response.StatusCode >= 400 {
		return nil, errChatSearchFailed
	}

	var parsed chatSearchResponse
	if err := json.NewDecoder(searchResp.Inspect().Response.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	passages := make([]llm.Passage, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		passages = append(passages, llm.Passage{
			DocID: hit.ID,
			Title: hit.Source.Title,
			Text:  hit.Source.Text,
		})
	}

	return passages, nil
}
