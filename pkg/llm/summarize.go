package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// KeyFactLLM is a single label/value pair extracted from a document.
type KeyFactLLM struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Summary is the result of summarizing a document.
type Summary struct {
	Text     string       `json:"summary"`
	KeyFacts []KeyFactLLM `json:"keyFacts"`
}

func (c *Client) summarizePrompt(text string) string {
	promptText := strings.TrimSpace(text)
	if len([]rune(promptText)) > maxInputRunes {
		promptText = string([]rune(promptText)[:maxInputRunes])
	}

	return fmt.Sprintf(`Given the full text of a single document, produce a short summary and extract key facts.

Return ONLY a strict JSON object with keys "summary" and "keyFacts".
- "summary": a 2-3 sentence TL;DR of the document.
- "keyFacts": an array of {"label":"...","value":"..."} objects extracting important values such as amount due, due date, IBAN, and reference numbers when present. Omit facts that are not present.

Example:
{"summary":"Electricity invoice for March 2026 from the local utility. Payment of CHF 84.20 is due by 2026-04-15.","keyFacts":[{"label":"Amount due","value":"CHF 84.20"},{"label":"Due date","value":"2026-04-15"},{"label":"Reference","value":"21 00000 00003 13947 14300 09017"}]}

Document text:
%s`, promptText)
}

func (c *Client) summarizeMessages(text string) []chatMessage {
	return []chatMessage{
		{
			Role:    "system",
			Content: "You are a strict document summarizer and fact extractor. Do not add explanations.",
		},
		{
			Role:    "user",
			Content: c.summarizePrompt(text),
		},
	}
}

func (c *Client) summarizeRequestBody(text string) ([]byte, error) {
	payload := ollamaRequest{
		Model:    c.model,
		Messages: c.summarizeMessages(text),
		Stream:   false,
		Format:   "json",
		Options: struct {
			Temperature float64 `json:"temperature"`
		}{Temperature: 0},
	}
	return json.Marshal(payload)
}

func cleanSummary(s Summary) Summary {
	s.Text = strings.TrimSpace(s.Text)

	var facts []KeyFactLLM
	for _, f := range s.KeyFacts {
		label := strings.TrimSpace(f.Label)
		value := strings.TrimSpace(f.Value)
		if label == "" && value == "" {
			continue
		}
		facts = append(facts, KeyFactLLM{Label: label, Value: value})
	}
	s.KeyFacts = facts
	return s
}

func (c *Client) parseSummary(rawContent string) (Summary, error) {
	obj, err := extractJSONObject(rawContent)
	if err != nil {
		return Summary{}, err
	}
	var result Summary
	if err := json.Unmarshal([]byte(obj), &result); err != nil {
		return Summary{}, err
	}
	return cleanSummary(result), nil
}

// Summarize produces a short TL;DR and a set of key facts for a document.
// An empty input returns a zero Summary and a nil error.
func (c *Client) Summarize(ctx context.Context, text string) (Summary, error) {
	if strings.TrimSpace(text) == "" {
		return Summary{}, nil
	}
	payload, err := c.summarizeRequestBody(text)
	if err != nil {
		return Summary{}, fmt.Errorf("marshal request: %w", err)
	}

	rawResponse, err := c.doRequest(ctx, c.chatURL(), payload)
	if err != nil {
		return Summary{}, fmt.Errorf("call llm: %w", err)
	}
	content, err := c.chatResponseContent(rawResponse)
	if err != nil {
		return Summary{}, fmt.Errorf("decode llm response: %w", err)
	}
	s, err := c.parseSummary(content)
	if err != nil {
		log.Warnf("LLM returned unparsable summary JSON: %q", strings.TrimSpace(content))
		return Summary{}, fmt.Errorf("parse summary: %w", err)
	}
	return s, nil
}
