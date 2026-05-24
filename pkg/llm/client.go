package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultLLMModel is the default Gemma model tag used when llm-model is not provided.
	// For CPU inference, use a quantized Gemma 4 variant such as:
	//   - gemma4:e2b        (2B active params, ~4 GB at Q4_K_M) — fastest on CPU
	//   - gemma4:e4b        (4B active params, ~6 GB at Q4_K_M) — better quality
	//   - gemma4:26b-a4b    (MoE, 4B active, ~16-18 GB at Q4_K_M)
	// Ollama model names are used here because the default client talks to Ollama.
	DefaultLLMModel  = "gemma4:e2b"
	defaultTimeout   = 45 * time.Second
	maxInputRunes    = 12000
	defaultMaxTokens = 256
)

var log = logrus.StandardLogger().WithField("package", "llm")

type Provider string

const (
	ProviderOllama Provider = "ollama"
)

type Metadata struct {
	Title   string `json:"title"`
	Company string `json:"company"`
}

type Option func(*Client)

func WithModel(model string) Option {
	return func(c *Client) {
		if model != "" {
			c.model = model
		}
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.http.Timeout = timeout
		}
	}
}

type Client struct {
	endpoint *url.URL
	model    string
	http     *http.Client
}

func New(endpoint string, opts ...Option) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}

	c := &Client{
		endpoint: u,
		model:    DefaultLLMModel,
		http: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format"`
	Options  struct {
		Temperature float64 `json:"temperature"`
	} `json:"options"`
}

type ollamaResponse struct {
	Message chatMessage `json:"message"`
}

func (c *Client) basePath() string {
	return strings.TrimRight(c.endpoint.String(), "/")
}

func (c *Client) extractPrompt(text string) string {
	promptText := strings.TrimSpace(text)
	if len([]rune(promptText)) > maxInputRunes {
		promptText = string([]rune(promptText)[:maxInputRunes])
	}

	return fmt.Sprintf(`Given the full text of a single document, extract:
1) subject/title
2) sender/publisher company

Return ONLY a strict JSON object with keys "title" and "company".
Use an empty string when a value is not certain.

Example:
{"title":"Vacanze autunnali, visite di città, gite nel weekend","company":"Touring Club Svizzero"}

Document text:
%s`, promptText)
}

func (c *Client) promptMessages(text string) []chatMessage {
	return []chatMessage{
		{
			Role:    "system",
			Content: "You are a strict document metadata extractor. Do not add explanations.",
		},
		{
			Role:    "user",
			Content: c.extractPrompt(text),
		},
	}
}

func (c *Client) chatRequestBody(text string) ([]byte, error) {
	payload := ollamaRequest{
		Model:    c.model,
		Messages: c.promptMessages(text),
		Stream:   false,
		Format:   "json",
		Options: struct {
			Temperature float64 `json:"temperature"`
		}{Temperature: 0},
	}
	return json.Marshal(payload)
}

func (c *Client) chatResponseContent(body []byte) (string, error) {
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err == nil && ollamaResp.Message.Content != "" {
		return ollamaResp.Message.Content, nil
	}
	return "", errors.New("unable to decode LLM chat response")
}

func extractJSONObject(raw string) (string, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", errors.New("no JSON object found in LLM response")
	}
	return strings.TrimSpace(raw[start : end+1]), nil
}

func cleanMetadata(meta Metadata) Metadata {
	meta.Title = strings.TrimSpace(meta.Title)
	meta.Company = strings.TrimSpace(meta.Company)
	if meta.Title == "" && meta.Company == "" {
		return Metadata{}
	}
	return meta
}

func (c *Client) parseMetadata(rawContent string) (Metadata, error) {
	obj, err := extractJSONObject(rawContent)
	if err != nil {
		return Metadata{}, err
	}
	var result Metadata
	if err := json.Unmarshal([]byte(obj), &result); err != nil {
		return Metadata{}, err
	}
	return cleanMetadata(result), nil
}

func (c *Client) chatURL() string {
	return c.basePath() + "/api/chat"
}

func (c *Client) healthURL() string {
	return c.basePath() + "/api/tags"
}

func (c *Client) doRequest(ctx context.Context, endpoint string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("unexpected status %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	return io.ReadAll(res.Body)
}

func (c *Client) ExtractMetadata(ctx context.Context, text string) (Metadata, error) {
	if strings.TrimSpace(text) == "" {
		return Metadata{}, nil
	}
	payload, err := c.chatRequestBody(text)
	if err != nil {
		return Metadata{}, fmt.Errorf("marshal request: %w", err)
	}

	rawResponse, err := c.doRequest(ctx, c.chatURL(), payload)
	if err != nil {
		return Metadata{}, fmt.Errorf("call llm: %w", err)
	}
	content, err := c.chatResponseContent(rawResponse)
	if err != nil {
		return Metadata{}, fmt.Errorf("decode llm response: %w", err)
	}
	meta, err := c.parseMetadata(content)
	if err != nil {
		log.Warnf("LLM returned unparsable JSON: %q", strings.TrimSpace(content))
		return Metadata{}, fmt.Errorf("parse metadata: %w", err)
	}
	return meta, nil
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.healthURL(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("perform health request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy status: %s", res.Status)
	}
	return nil
}
