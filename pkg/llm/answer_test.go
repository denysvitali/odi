package llm

import (
	"context"
	"strings"
	"testing"
)

func TestAnswerNoPassagesSkipsLLM(t *testing.T) {
	// With no passages, Answer must return a canned message without dialing the
	// (here zero-value, unusable) endpoint.
	c := &Client{}
	got, err := c.Answer(context.Background(), "anything?", nil)
	if err != nil {
		t.Fatalf("Answer(no passages): unexpected error %v", err)
	}
	if got != "I could not find relevant documents." {
		t.Errorf("unexpected fallback answer: %q", got)
	}
}

func TestBuildAnswerPrompt(t *testing.T) {
	prompt := buildAnswerPrompt("How much is due?", []Passage{
		{DocID: "1", Title: "Electricity bill", Text: "Total CHF 120"},
		{DocID: "2", Title: "", Text: "second excerpt"},
	})

	if !strings.Contains(prompt, "Question: How much is due?") {
		t.Errorf("prompt missing question: %q", prompt)
	}
	if !strings.Contains(prompt, "[1] Electricity bill") {
		t.Errorf("prompt missing numbered first excerpt: %q", prompt)
	}
	// A passage with a blank title falls back to "Untitled".
	if !strings.Contains(prompt, "[2] Untitled") {
		t.Errorf("prompt missing Untitled fallback: %q", prompt)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Errorf("short string changed: %q", got)
	}
	if got := truncateRunes("hello", 3); got != "hel" {
		t.Errorf("truncation: got %q, want %q", got, "hel")
	}
	// Multi-byte runes must be counted as single runes, not bytes.
	if got := truncateRunes("héllo", 2); got != "hé" {
		t.Errorf("rune-aware truncation: got %q, want %q", got, "hé")
	}
}

func TestBuildAnswerPromptCapsLength(t *testing.T) {
	long := strings.Repeat("x", maxInputRunes*2)
	prompt := buildAnswerPrompt("q", []Passage{{Text: long}})
	if len([]rune(prompt)) > maxInputRunes {
		t.Errorf("prompt exceeds maxInputRunes: %d > %d", len([]rune(prompt)), maxInputRunes)
	}
}
