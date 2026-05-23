package b2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScannedPageFromRemote(t *testing.T) {
	tests := []struct {
		name       string
		remote     string
		wantOK     bool
		wantScanID string
		wantSeqID  int
	}{
		{name: "page", remote: "scan-1/2.jpg", wantOK: true, wantScanID: "scan-1", wantSeqID: 2},
		{name: "thumbnail", remote: "scan-1/2_thumb.jpg", wantOK: false},
		{name: "root file", remote: "2.jpg", wantOK: false},
		{name: "not jpg", remote: "scan-1/2.png", wantOK: false},
		{name: "bad sequence", remote: "scan-1/front.jpg", wantOK: false},
		{name: "zero sequence", remote: "scan-1/0.jpg", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, ok := scannedPageFromRemote(tt.remote)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantScanID, page.ScanID)
				assert.Equal(t, tt.wantSeqID, page.SequenceID)
			}
		})
	}
}
