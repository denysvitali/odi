package fs

import (
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/odi/pkg/models"
)

func TestValidateScanID(t *testing.T) {
	tests := []struct {
		name    string
		scanID  string
		wantErr bool
	}{
		{"valid alphanumeric", "scan123", false},
		{"valid with hyphen", "scan-123", false},
		{"valid with underscore", "scan_123", false},
		{"valid mixed", "Scan_123-abc", false},
		{"empty", "", true},
		{"path traversal dotdot", "..", true},
		{"path traversal with slash", "../etc/passwd", true},
		{"path with slash", "foo/bar", true},
		{"path with backslash", `foo\bar`, true},
		{"dot file", ".hidden", true},
		{"spaces", "scan id", true},
		{"special chars", "scan@123", true},
		{"unicode", "scaén", true},
		{"newline", "scan\n", true},
		{"null byte", "scan\x00", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateScanID(tc.scanID)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNew_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "nested", "storage")
	store, err := New(target)
	require.NoError(t, err)
	require.NotNil(t, store)
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNew_ExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, store.dir)
}

func TestStoreAndRetrieve(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	content := []byte("hello world")
	page := models.ScannedPage{
		ScanID:     "abc-123",
		SequenceID: 7,
		Reader:     bytes.NewReader(content),
	}

	require.NoError(t, store.Store(context.Background(), page))

	// File exists at expected path
	expectedPath := path.Join(dir, "abc-123", "7.jpg")
	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Reader was rewound
	rewound, err := io.ReadAll(page.Reader)
	require.NoError(t, err)
	assert.Equal(t, content, rewound)

	// Retrieve gives us back the bytes
	got, err := store.Retrieve(context.Background(), "abc-123", 7)
	require.NoError(t, err)
	require.NotNil(t, got)
	t.Cleanup(func() {
		if closer, ok := got.Reader.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	out, err := io.ReadAll(got.Reader)
	require.NoError(t, err)
	assert.Equal(t, content, out)
	assert.Equal(t, "abc-123", got.ScanID)
	assert.Equal(t, 7, got.SequenceID)
}

func TestStore_RejectsBadScanID(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	badIDs := []string{"", "..", "../escape", "with/slash", "bad name"}
	for _, id := range badIDs {
		t.Run(id, func(t *testing.T) {
			err := store.Store(context.Background(), models.ScannedPage{
				ScanID:     id,
				SequenceID: 0,
				Reader:     bytes.NewReader([]byte("x")),
			})
			assert.Error(t, err)
		})
	}
}

func TestRetrieve_RejectsBadScanID(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	_, err = store.Retrieve(context.Background(), "", 0)
	assert.Error(t, err)
	_, err = store.Retrieve(context.Background(), "../etc", 0)
	assert.Error(t, err)
}

func TestRetrieve_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	_, err = store.Retrieve(context.Background(), "nonexistent", 0)
	assert.Error(t, err)
}

func TestStore_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	first := []byte("first content longer")
	require.NoError(t, store.Store(context.Background(), models.ScannedPage{
		ScanID: "doc", SequenceID: 0, Reader: bytes.NewReader(first),
	}))

	second := []byte("second")
	require.NoError(t, store.Store(context.Background(), models.ScannedPage{
		ScanID: "doc", SequenceID: 0, Reader: bytes.NewReader(second),
	}))

	data, err := os.ReadFile(path.Join(dir, "doc", "0.jpg"))
	require.NoError(t, err)
	assert.Equal(t, second, data, "file should be truncated and replaced")
}

func TestListPages(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), models.ScannedPage{
		ScanID: "scan-a", SequenceID: 1, Reader: bytes.NewReader([]byte("one")),
	}))
	require.NoError(t, store.Store(context.Background(), models.ScannedPage{
		ScanID: "scan-a", SequenceID: 2, Reader: bytes.NewReader([]byte("two")),
	}))
	require.NoError(t, os.WriteFile(path.Join(dir, "scan-a", "2_thumb.jpg"), []byte("thumb"), 0600))
	require.NoError(t, os.WriteFile(path.Join(dir, "scan-a", "front.jpg"), []byte("bad"), 0600))

	pages, err := store.ListPages(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []models.ScannedPage{
		{ScanID: "scan-a", SequenceID: 1},
		{ScanID: "scan-a", SequenceID: 2},
	}, pages)
}
