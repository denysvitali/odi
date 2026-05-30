package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Classification is the result of classifying a document into a type and tags.
type Classification struct {
	DocType string   `json:"docType"`
	Tags    []string `json:"tags"`
}

// validDocTypes is the closed enum of allowed document types. Anything outside
// this set is normalized to "other".
var validDocTypes = map[string]struct{}{
	"invoice":        {},
	"contract":       {},
	"payslip":        {},
	"tax":            {},
	"insurance":      {},
	"medical":        {},
	"correspondence": {},
	"receipt":        {},
	"bank":           {},
	"other":          {},
}

const maxTags = 5

func (c *Client) classifyPrompt(text string) string {
	promptText := strings.TrimSpace(text)
	if len([]rune(promptText)) > maxInputRunes {
		promptText = string([]rune(promptText)[:maxInputRunes])
	}

	return fmt.Sprintf(`Given the full text of a single document, classify it.

Return ONLY a strict JSON object with keys "docType" and "tags".
- "docType" MUST be exactly one of: "invoice", "contract", "payslip", "tax", "insurance", "medical", "correspondence", "receipt", "bank", "other".
- "tags" MUST be an array of 3-5 short lowercase topical tags.

Example:
{"docType":"invoice","tags":["electricity","utility","monthly","payment"]}

Document text:
%s`, promptText)
}

func (c *Client) classifyMessages(text string) []chatMessage {
	return []chatMessage{
		{
			Role:    "system",
			Content: "You are a strict document classifier. Do not add explanations.",
		},
		{
			Role:    "user",
			Content: c.classifyPrompt(text),
		},
	}
}

func (c *Client) classifyRequestBody(text string) ([]byte, error) {
	payload := ollamaRequest{
		Model:    c.model,
		Messages: c.classifyMessages(text),
		Stream:   false,
		Format:   "json",
		Options: struct {
			Temperature float64 `json:"temperature"`
		}{Temperature: 0},
	}
	return json.Marshal(payload)
}

func cleanClassification(cl Classification) Classification {
	docType := strings.ToLower(strings.TrimSpace(cl.DocType))
	if _, ok := validDocTypes[docType]; !ok {
		docType = "other"
	}

	var tags []string
	seen := make(map[string]struct{})
	for _, t := range cl.Tags {
		tag := strings.ToLower(strings.TrimSpace(t))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
		if len(tags) >= maxTags {
			break
		}
	}

	return Classification{DocType: docType, Tags: tags}
}

func (c *Client) parseClassification(rawContent string) (Classification, error) {
	obj, err := extractJSONObject(rawContent)
	if err != nil {
		return Classification{}, err
	}
	var result Classification
	if err := json.Unmarshal([]byte(obj), &result); err != nil {
		return Classification{}, err
	}
	return cleanClassification(result), nil
}

// Classify determines the document type and a small set of topical tags.
// An empty input returns a zero Classification and a nil error.
func (c *Client) Classify(ctx context.Context, text string) (Classification, error) {
	if strings.TrimSpace(text) == "" {
		return Classification{}, nil
	}
	payload, err := c.classifyRequestBody(text)
	if err != nil {
		return Classification{}, fmt.Errorf("marshal request: %w", err)
	}

	rawResponse, err := c.doRequest(ctx, c.chatURL(), payload)
	if err != nil {
		return Classification{}, fmt.Errorf("call llm: %w", err)
	}
	content, err := c.chatResponseContent(rawResponse)
	if err != nil {
		return Classification{}, fmt.Errorf("decode llm response: %w", err)
	}
	cl, err := c.parseClassification(content)
	if err != nil {
		log.Warnf("LLM returned unparsable classification JSON: %q", strings.TrimSpace(content))
		return Classification{}, fmt.Errorf("parse classification: %w", err)
	}
	return cl, nil
}
