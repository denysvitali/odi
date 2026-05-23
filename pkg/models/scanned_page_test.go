package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScannedPage_ID(t *testing.T) {
	tests := []struct {
		name    string
		scanID  string
		seqID   int
		wantStr string
	}{
		{"simple", "abc", 0, "abc_0"},
		{"with hyphen", "scan-1", 12, "scan-1_12"},
		{"empty scan", "", 5, "_5"},
		{"negative sequence", "doc", -1, "doc_-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := ScannedPage{ScanID: tc.scanID, SequenceID: tc.seqID}
			assert.Equal(t, tc.wantStr, s.ID())
		})
	}
}
