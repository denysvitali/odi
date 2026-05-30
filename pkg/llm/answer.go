package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// answerSystemPrompt instructs the model to answer strictly from the supplied
// document excerpts and to admit when it cannot find an answer.
const answerSystemPrompt = "You answer questions strictly from the provided document excerpts. " +
	"Cite nothing you cannot support. If the excerpts do not contain the answer, say you could not find it."

// maxPassageRunes caps the length of a single excerpt so that one long
// document cannot crowd out the others within the overall maxInputRunes
// budget for the prompt.
const maxPassageRunes = 1500

// Passage is a single document excerpt used to ground an Answer call.
type Passage struct {
	DocID string
	Title string
	Text  string
}

// truncateRunes returns s truncated to at most n runes.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// buildAnswerPrompt renders the user message: the question followed by the
// numbered excerpts. Each excerpt is truncated individually and the whole
// prompt is capped at maxInputRunes.
func buildAnswerPrompt(question string, passages []Passage) string {
	var b strings.Builder
	b.WriteString("Question: ")
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\n\nDocument excerpts:\n")

	for i, p := range passages {
		title := strings.TrimSpace(p.Title)
		if title == "" {
			title = "Untitled"
		}
		text := truncateRunes(strings.TrimSpace(p.Text), maxPassageRunes)
		fmt.Fprintf(&b, "\n[%d] %s\n%s\n", i+1, title, text)
	}

	return truncateRunes(b.String(), maxInputRunes)
}

// Answer produces a grounded, prose answer to question using only the supplied
// passages. When no passages are provided it returns a graceful message
// without contacting the LLM.
func (c *Client) Answer(ctx context.Context, question string, passages []Passage) (string, error) {
	if len(passages) == 0 {
		return "I could not find relevant documents.", nil
	}

	payload := ollamaRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: answerSystemPrompt},
			{Role: "user", Content: buildAnswerPrompt(question, passages)},
		},
		Stream: false,
		Format: "",
		Options: struct {
			Temperature float64 `json:"temperature"`
		}{Temperature: 0},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	rawResponse, err := c.doRequest(ctx, c.chatURL(), body)
	if err != nil {
		return "", fmt.Errorf("call llm: %w", err)
	}

	content, err := c.chatResponseContent(rawResponse)
	if err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}

	return strings.TrimSpace(content), nil
}
