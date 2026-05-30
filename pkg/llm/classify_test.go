package llm

import (
	"context"
	"testing"
)

func TestCleanClassificationNormalizesDocType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"invoice", "invoice"},
		{"  Invoice ", "invoice"},
		{"INVOICE", "invoice"},
		{"unknown-type", "other"},
		{"", "other"},
	}
	for _, tt := range tests {
		got := cleanClassification(Classification{DocType: tt.in})
		if got.DocType != tt.want {
			t.Errorf("docType %q: got %q, want %q", tt.in, got.DocType, tt.want)
		}
	}
}

func TestCleanClassificationTags(t *testing.T) {
	got := cleanClassification(Classification{
		DocType: "invoice",
		Tags:    []string{"Utility", "utility", " Monthly ", "", "a", "b", "c", "d"},
	})

	// Deduped (Utility/utility collapse), lowercased, trimmed, capped at maxTags.
	if len(got.Tags) > maxTags {
		t.Fatalf("tags exceed cap: got %d, want <= %d (%v)", len(got.Tags), maxTags, got.Tags)
	}
	if got.Tags[0] != "utility" {
		t.Errorf("first tag not lowercased/deduped: %v", got.Tags)
	}
	for _, tag := range got.Tags {
		if tag == "" {
			t.Errorf("empty tag should have been dropped: %v", got.Tags)
		}
	}
}

func TestParseClassificationFromMessyResponse(t *testing.T) {
	c := &Client{}
	raw := "Sure! Here is the JSON:\n{\"docType\":\"receipt\",\"tags\":[\"grocery\",\"food\"]}\nHope that helps."
	got, err := c.parseClassification(raw)
	if err != nil {
		t.Fatalf("parseClassification: %v", err)
	}
	if got.DocType != "receipt" {
		t.Errorf("docType: got %q, want receipt", got.DocType)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags: got %v, want 2 entries", got.Tags)
	}
}

func TestClassifyEmptyInputSkipsLLM(t *testing.T) {
	// A nil-endpoint client must not be dialed for empty/whitespace input.
	c := &Client{}
	got, err := c.Classify(context.Background(), "   ")
	if err != nil {
		t.Fatalf("Classify(empty): unexpected error %v", err)
	}
	if got.DocType != "" || len(got.Tags) != 0 {
		t.Errorf("Classify(empty): expected zero value, got %+v", got)
	}
}
