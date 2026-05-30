package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// helper to create a temp directory that is cleaned up after the test.
func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// writeTestFile creates a file at path with the given content.
func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

// jpegHeader is the minimal magic bytes for a JPEG file.
var jpegHeader = []byte{0xFF, 0xD8, 0xFF, 0xE0}

// pngHeader is the minimal magic bytes for a PNG file.
var pngHeader = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// ---------------------------------------------------------------------------
// Test file detection: create a file, verify it is picked up
// ---------------------------------------------------------------------------

func TestFileDetection(t *testing.T) {
	dir := tempDir(t)

	var processed []string
	var mu sync.Mutex
	processor := func(path string) error {
		mu.Lock()
		processed = append(processed, filepath.Base(path))
		mu.Unlock()
		return nil
	}

	w, err := New(Config{
		Dir:      dir,
		Debounce: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	// Give the watcher a moment to register.
	time.Sleep(100 * time.Millisecond)

	writeTestFile(t, filepath.Join(dir, "test.jpg"), jpegHeader)

	// Wait for debounce + processing.
	time.Sleep(300 * time.Millisecond)

	cancel()
	_ = w.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(processed) == 0 {
		t.Fatal("expected file to be processed, but processor was never called")
	}
	if processed[0] != "test.jpg" {
		t.Errorf("expected test.jpg, got %s", processed[0])
	}
}

// ---------------------------------------------------------------------------
// Test debouncing: rapid modifications should produce only one event
// ---------------------------------------------------------------------------

func TestDebouncing(t *testing.T) {
	dir := tempDir(t)

	var callCount atomic.Int32
	processor := func(path string) error {
		callCount.Add(1)
		return nil
	}

	w, err := New(Config{
		Dir:      dir,
		Debounce: 150 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	// Write a file, then modify it rapidly multiple times.
	path := filepath.Join(dir, "rapid.txt")
	writeTestFile(t, path, []byte("v1"))
	for i := 0; i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		writeTestFile(t, path, []byte(fmt.Sprintf("v%d", i+2)))
	}

	// Wait longer than debounce + some buffer.
	time.Sleep(500 * time.Millisecond)

	cancel()
	_ = w.Stop()

	count := callCount.Load()
	if count != 1 {
		t.Errorf("expected exactly 1 processor call after debouncing, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Test MIME type filtering: .txt ignored, .jpg accepted
// ---------------------------------------------------------------------------

func TestMIMETypeFiltering(t *testing.T) {
	dir := tempDir(t)

	var processed []string
	var mu sync.Mutex
	processor := func(path string) error {
		mu.Lock()
		processed = append(processed, filepath.Base(path))
		mu.Unlock()
		return nil
	}

	w, err := New(Config{
		Dir:          dir,
		Debounce:     50 * time.Millisecond,
		AllowedMIMEs: []string{"image/jpeg"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	// Create a .txt file (should be ignored) and a JPEG (should be accepted).
	writeTestFile(t, filepath.Join(dir, "notes.txt"), []byte("hello"))
	time.Sleep(50 * time.Millisecond)
	writeTestFile(t, filepath.Join(dir, "photo.jpg"), jpegHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	mu.Lock()
	defer mu.Unlock()

	for _, name := range processed {
		if name == "notes.txt" {
			t.Error("txt file should have been filtered out by MIME type")
		}
	}

	found := false
	for _, name := range processed {
		if name == "photo.jpg" {
			found = true
		}
	}
	if !found {
		t.Error("jpeg file should have been processed")
	}
}

// ---------------------------------------------------------------------------
// Test .done/ directory movement after processing
// ---------------------------------------------------------------------------

func TestDoneDirectoryMovement(t *testing.T) {
	dir := tempDir(t)

	processor := func(path string) error {
		return nil
	}

	w, err := New(Config{
		Dir:      dir,
		Debounce: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	fileName := "receipt.jpg"
	origPath := filepath.Join(dir, fileName)
	writeTestFile(t, origPath, jpegHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	// Original should no longer exist.
	if _, err := os.Stat(origPath); !os.IsNotExist(err) {
		t.Errorf("original file should have been moved, but still exists at %s", origPath)
	}

	// File should now be in .done/.
	donePath := filepath.Join(w.DoneDir(), fileName)
	if _, err := os.Stat(donePath); err != nil {
		t.Errorf("file should exist in .done/ at %s: %v", donePath, err)
	}
}

// ---------------------------------------------------------------------------
// Test recursive vs non-recursive mode
// ---------------------------------------------------------------------------

func TestRecursiveMode(t *testing.T) {
	dir := tempDir(t)
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var processed []string
	var mu sync.Mutex
	processor := func(path string) error {
		mu.Lock()
		processed = append(processed, path)
		mu.Unlock()
		return nil
	}

	w, err := New(Config{
		Dir:       dir,
		Recursive: true,
		Debounce:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(200 * time.Millisecond)

	writeTestFile(t, filepath.Join(subDir, "deep.jpg"), jpegHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(processed) == 0 {
		t.Fatal("recursive mode should detect files in subdirectories")
	}
}

func TestNonRecursiveMode(t *testing.T) {
	dir := tempDir(t)
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var processed []string
	var mu sync.Mutex
	processor := func(path string) error {
		mu.Lock()
		processed = append(processed, filepath.Base(path))
		mu.Unlock()
		return nil
	}

	w, err := New(Config{
		Dir:       dir,
		Recursive: false,
		Debounce:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(200 * time.Millisecond)

	// Write in subdir -- should NOT be picked up in non-recursive mode.
	writeTestFile(t, filepath.Join(subDir, "deep.jpg"), jpegHeader)
	time.Sleep(100 * time.Millisecond)

	// Write in root -- SHOULD be picked up.
	writeTestFile(t, filepath.Join(dir, "root.jpg"), jpegHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	mu.Lock()
	defer mu.Unlock()

	for _, name := range processed {
		if name == "deep.jpg" {
			t.Error("non-recursive mode should NOT detect files in subdirectories")
		}
	}

	found := false
	for _, name := range processed {
		if name == "root.jpg" {
			found = true
		}
	}
	if !found {
		t.Error("non-recursive mode should detect files in the root directory")
	}
}

// ---------------------------------------------------------------------------
// Test duplicate file handling (file already in .done/)
// ---------------------------------------------------------------------------

func TestDuplicateHandling(t *testing.T) {
	dir := tempDir(t)
	doneDir := filepath.Join(dir, ".done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-create a file in .done/ to simulate it was already processed.
	writeTestFile(t, filepath.Join(doneDir, "dup.jpg"), jpegHeader)

	var callCount atomic.Int32
	processor := func(path string) error {
		callCount.Add(1)
		return nil
	}

	w, err := New(Config{
		Dir:      dir,
		Debounce: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	// Create a file with the same name as one already in .done/.
	writeTestFile(t, filepath.Join(dir, "dup.jpg"), jpegHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	count := callCount.Load()
	if count != 0 {
		t.Errorf("duplicate file should not be processed, but processor was called %d times", count)
	}
}

// ---------------------------------------------------------------------------
// Test that processor errors do not move the file to .done/
// ---------------------------------------------------------------------------

func TestProcessorErrorPreventsMove(t *testing.T) {
	dir := tempDir(t)

	processor := func(path string) error {
		return fmt.Errorf("simulated failure")
	}

	w, err := New(Config{
		Dir:      dir,
		Debounce: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	fileName := "fail.jpg"
	origPath := filepath.Join(dir, fileName)
	writeTestFile(t, origPath, jpegHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	// The file should still exist at the original location since processing failed.
	if _, err := os.Stat(origPath); os.IsNotExist(err) {
		t.Error("file should NOT be moved to .done/ when processor returns an error")
	}
}

// ---------------------------------------------------------------------------
// Test PNG MIME type is accepted when image/png is allowed
// ---------------------------------------------------------------------------

func TestMIMETypeFiltering_PNGAccepted(t *testing.T) {
	dir := tempDir(t)

	var processed []string
	var mu sync.Mutex
	processor := func(path string) error {
		mu.Lock()
		processed = append(processed, filepath.Base(path))
		mu.Unlock()
		return nil
	}

	w, err := New(Config{
		Dir:          dir,
		Debounce:     50 * time.Millisecond,
		AllowedMIMEs: []string{"image/png"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	writeTestFile(t, filepath.Join(dir, "image.png"), pngHeader)

	time.Sleep(400 * time.Millisecond)

	cancel()
	_ = w.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(processed) == 0 {
		t.Fatal("png file should be processed when image/png is allowed")
	}
	if processed[0] != "image.png" {
		t.Errorf("expected image.png, got %s", processed[0])
	}
}

// ---------------------------------------------------------------------------
// Test multiple allowed MIME types
// ---------------------------------------------------------------------------

func TestMIMETypeFiltering_MultipleAllowed(t *testing.T) {
	dir := tempDir(t)

	var processed []string
	var mu sync.Mutex
	processor := func(path string) error {
		mu.Lock()
		processed = append(processed, filepath.Base(path))
		mu.Unlock()
		return nil
	}

	w, err := New(Config{
		Dir:          dir,
		Debounce:     50 * time.Millisecond,
		AllowedMIMEs: []string{"image/jpeg", "image/png"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Start(ctx, processor) }()

	time.Sleep(100 * time.Millisecond)

	writeTestFile(t, filepath.Join(dir, "a.jpg"), jpegHeader)
	time.Sleep(50 * time.Millisecond)
	writeTestFile(t, filepath.Join(dir, "b.png"), pngHeader)
	time.Sleep(50 * time.Millisecond)
	writeTestFile(t, filepath.Join(dir, "c.txt"), []byte("text"))

	time.Sleep(500 * time.Millisecond)

	cancel()
	_ = w.Stop()

	mu.Lock()
	defer mu.Unlock()

	for _, name := range processed {
		if name == "c.txt" {
			t.Error("txt file should not be processed")
		}
	}

	if len(processed) < 2 {
		t.Errorf("expected at least 2 files processed, got %d", len(processed))
	}
}

// ---------------------------------------------------------------------------
// Test New() with empty Dir returns error
// ---------------------------------------------------------------------------

func TestNew_EmptyDir(t *testing.T) {
	_, err := New(Config{Dir: ""})
	if err == nil {
		t.Fatal("expected error when Dir is empty")
	}
}

// ---------------------------------------------------------------------------
// Test default debounce value
// ---------------------------------------------------------------------------

func TestDefaultDebounce(t *testing.T) {
	cfg := Config{Dir: "/tmp/test"}
	if cfg.debounce() != 500*time.Millisecond {
		t.Errorf("expected default debounce of 500ms, got %v", cfg.debounce())
	}

	cfg.Debounce = 1 * time.Second
	if cfg.debounce() != 1*time.Second {
		t.Errorf("expected custom debounce of 1s, got %v", cfg.debounce())
	}
}
