package contentdigest

import "testing"

func TestSumReturnsStableSHA256Digest(t *testing.T) {
	got := Sum([]byte("same image bytes"))
	want := "sha256:f10266197016b8e8842aeba6800100997ce04f35a45a3bff974711e9615ea597"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
