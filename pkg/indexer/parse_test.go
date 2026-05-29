package indexer

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/denysvitali/odi/pkg/ocrclient"
)

// ocrResultWithBarcodes builds an *ocrclient.OcrResult from the given raw
// barcode values. The barcode element type is unexported, so we round-trip
// through JSON (the OcrResult.Barcodes field and rawValue tag are exported).
func ocrResultWithBarcodes(t *testing.T, rawValues ...string) *ocrclient.OcrResult {
	t.Helper()
	type bc struct {
		RawValue string `json:"rawValue"`
	}
	payload := struct {
		Barcodes []bc `json:"barcodes"`
	}{}
	for _, v := range rawValues {
		payload.Barcodes = append(payload.Barcodes, bc{RawValue: v})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var res ocrclient.OcrResult
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatalf("unmarshal OcrResult: %v", err)
	}
	return &res
}

func TestGetBarcodesNilResult(t *testing.T) {
	i := &Indexer{}
	if got := i.getBarcodes(nil); got != nil {
		t.Fatalf("expected nil for nil result, got %#v", got)
	}
}

func TestGetBarcodesPlainText(t *testing.T) {
	i := &Indexer{}
	res := ocrResultWithBarcodes(t, "https://example.com", "1234567890")

	got := i.getBarcodes(res)
	if len(got) != 2 {
		t.Fatalf("expected 2 barcodes, got %d", len(got))
	}
	for idx, want := range []string{"https://example.com", "1234567890"} {
		if got[idx].Text != want {
			t.Errorf("barcode %d: Text = %q, want %q", idx, got[idx].Text, want)
		}
		if got[idx].QRBill != nil {
			t.Errorf("barcode %d: expected no QRBill for plain text", idx)
		}
	}
}

func TestGetBarcodesInvalidSwissQRFallsBackToText(t *testing.T) {
	// "SPC" prefix marks a Swiss QR Bill, but a malformed payload must not be
	// dropped: it falls back to a plain-text barcode carrying the raw value.
	raw := "SPC\nnot-a-valid-qr-bill"
	i := &Indexer{}
	got := i.getBarcodes(ocrResultWithBarcodes(t, raw))

	if len(got) != 1 {
		t.Fatalf("expected 1 barcode, got %d", len(got))
	}
	if got[0].QRBill != nil {
		t.Errorf("expected no QRBill for malformed SPC payload")
	}
	if got[0].Text != raw {
		t.Errorf("Text = %q, want raw value %q", got[0].Text, raw)
	}
}

func TestDecodeError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"string error", `{"error":"boom"}`, "boom"},
		{"object error", `{"error":{"type":"x","reason":"y"}}`, `{"type":"x","reason":"y"}`},
		{"empty error field", `{}`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeError(io.NopCloser(strings.NewReader(tc.body)))
			if got != tc.want {
				t.Errorf("decodeError(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestDecodeErrorInvalidJSON(t *testing.T) {
	got := decodeError(io.NopCloser(strings.NewReader("not json")))
	if !strings.HasPrefix(got, "failed to decode error:") {
		t.Errorf("expected decode-failure message, got %q", got)
	}
}
