// Package watcher implements a filesystem watcher that monitors a directory for
// new or modified files, applies MIME-type filtering, debounces rapid changes,
// and moves processed files into a .done/ subdirectory.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

var log = logrus.StandardLogger().WithField("package", "watcher")

// Config controls the watcher behaviour.
type Config struct {
	// Dir is the root directory to watch.
	Dir string
	// Recursive watches subdirectories when true.
	Recursive bool
	// Debounce is the idle time after the last event before a file is processed.
	// Zero defaults to 500 ms.
	Debounce time.Duration
	// AllowedMIMEs is the set of accepted MIME types (e.g. "image/jpeg").
	// If empty, all types are accepted.
	AllowedMIMEs []string
}

func (c Config) debounce() time.Duration {
	if c.Debounce > 0 {
		return c.Debounce
	}
	return 500 * time.Millisecond
}

// FileProcessor is called for each file that passes filtering.
type FileProcessor func(path string) error

// Event represents a file-system event surfaced by the watcher.
type Event struct {
	Path string
	Op   fsnotify.Op
}

// Watcher monitors a directory and invokes a callback for qualifying files.
type Watcher struct {
	cfg     Config
	w       *fsnotify.Watcher
	mu      sync.Mutex
	pending map[string]*time.Timer
	doneDir string
	cancel  context.CancelFunc
	stopped chan struct{}
}

// New creates a Watcher from the given config. Call Start to begin monitoring.
func New(cfg Config) (*Watcher, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("watcher: Dir must not be empty")
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watcher: create fsnotify watcher: %w", err)
	}
	return &Watcher{
		cfg:     cfg,
		w:       w,
		pending: make(map[string]*time.Timer),
		doneDir: filepath.Join(cfg.Dir, ".done"),
		stopped: make(chan struct{}),
	}, nil
}

// Start begins watching and blocks until ctx is cancelled or Stop is called.
// It registers the root directory (and optionally subdirectories) and invokes
// processor for every qualifying file event.
func (wt *Watcher) Start(ctx context.Context, processor FileProcessor) error {
	ctx, wt.cancel = context.WithCancel(ctx)
	defer close(wt.stopped)

	if err := os.MkdirAll(wt.doneDir, 0o750); err != nil {
		return fmt.Errorf("watcher: create .done dir: %w", err)
	}

	if err := wt.addWatchPaths(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-wt.w.Events:
			if !ok {
				return nil
			}
			wt.handleEvent(evt, processor)
		case err, ok := <-wt.w.Errors:
			if !ok {
				return nil
			}
			log.Errorf("watcher error: %v", err)
		}
	}
}

// Stop signals the watcher to stop and waits for the run loop to exit.
func (wt *Watcher) Stop() error {
	if wt.cancel != nil {
		wt.cancel()
	}
	err := wt.w.Close()
	<-wt.stopped
	return err
}

func (wt *Watcher) addWatchPaths() error {
	if err := wt.w.Add(wt.cfg.Dir); err != nil {
		return fmt.Errorf("watcher: add %s: %w", wt.cfg.Dir, err)
	}
	if wt.cfg.Recursive {
		return filepath.Walk(wt.cfg.Dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() != ".done" {
				return wt.w.Add(path)
			}
			return nil
		})
	}
	return nil
}

func (wt *Watcher) handleEvent(evt fsnotify.Event, processor FileProcessor) {
	// Ignore events on the .done directory itself.
	if strings.HasPrefix(evt.Name, wt.doneDir) {
		return
	}

	// Only handle create and write events.
	if evt.Op&(fsnotify.Create|fsnotify.Write) == 0 {
		return
	}

	// Skip directories.
	info, err := os.Stat(evt.Name)
	if err != nil || info.IsDir() {
		return
	}

	// Debounce: cancel any existing timer for this path and start a new one.
	wt.mu.Lock()
	if t, ok := wt.pending[evt.Name]; ok {
		t.Stop()
	}
	wt.pending[evt.Name] = time.AfterFunc(wt.cfg.debounce(), func() {
		wt.mu.Lock()
		delete(wt.pending, evt.Name)
		wt.mu.Unlock()
		wt.processFile(evt.Name, processor)
	})
	wt.mu.Unlock()
}

func (wt *Watcher) processFile(path string, processor FileProcessor) {
	// Check duplicate: skip if already in .done.
	base := filepath.Base(path)
	if _, err := os.Stat(filepath.Join(wt.doneDir, base)); err == nil {
		log.Debugf("watcher: skipping duplicate %s", base)
		return
	}

	// MIME filter.
	if len(wt.cfg.AllowedMIMEs) > 0 {
		if !wt.isAllowedMIME(path) {
			log.Debugf("watcher: skipping %s (MIME not allowed)", base)
			return
		}
	}

	if err := processor(path); err != nil {
		log.Errorf("watcher: process %s: %v", base, err)
		return
	}

	// Move to .done.
	dest := filepath.Join(wt.doneDir, base)
	if err := os.Rename(path, dest); err != nil {
		log.Errorf("watcher: move %s to .done: %v", base, err)
	}
}

func (wt *Watcher) isAllowedMIME(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			mt, _, _ := mime.ParseMediaType(mimeType)
			for _, allowed := range wt.cfg.AllowedMIMEs {
				if mt == allowed {
					return true
				}
			}
			return false
		}
	}

	// Fallback: sniff the first 512 bytes.
	f, err := os.Open(path) //nolint:gosec // G304: path comes from fsnotify, not user input
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	detected := http.DetectContentType(buf[:n])
	if idx := strings.IndexByte(detected, ';'); idx >= 0 {
		detected = detected[:idx]
	}
	for _, allowed := range wt.cfg.AllowedMIMEs {
		if detected == allowed {
			return true
		}
	}
	return false
}

// DoneDir returns the path to the .done directory. Exposed for testing.
func (wt *Watcher) DoneDir() string {
	return wt.doneDir
}
