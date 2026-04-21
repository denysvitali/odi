package ingestor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"

	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
)

var log = logrus.StandardLogger()

const (
	DefaultWorkers = 10
	// PageScanDelay is the delay between scanning pages to prevent overwhelming the scanner
	PageScanDelay = 100 * time.Millisecond
)

// Config configures a local ingestor (storage + in-process OCR/indexing).
// For remote ingestion use NewWithBackend + NewRemoteBackend.
type Config struct {
	OcrAPIAddr         string
	OpenSearchAddr     string
	OpenSearchUsername string
	OpenSearchPassword string
	OpenSearchSkipTLS  bool
	OpenSearchIndex    string
	ZefixDsn           string
	Storage            model.Storer
}

type Ingestor struct {
	backend Backend
}

// New creates an Ingestor backed by a LocalBackend built from the given Config.
// This preserves the historical constructor signature.
func New(config Config) (*Ingestor, error) {
	b, err := NewLocalBackend(config)
	if err != nil {
		return nil, err
	}
	ing := NewWithBackend(b)
	log.Debugf("Pinging services")
	if err := ing.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to ping services: %w", err)
	}
	return ing, nil
}

// NewWithBackend creates an Ingestor that delegates page processing to the
// supplied Backend. Ping is NOT called automatically — the caller should do
// that explicitly if desired.
func NewWithBackend(b Backend) *Ingestor {
	return &Ingestor{backend: b}
}

func (i *Ingestor) ScanPages(ctx context.Context, scanner DocumentsScanner, workers int) error {
	if workers <= 0 {
		workers = DefaultWorkers
	}

	pageChan := make(chan models.ScannedPage, workers)
	wg := sync.WaitGroup{}
	var failedPages atomic.Int64
	var totalPages atomic.Int64

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go i.processPage(ctx, pageChan, &wg, &failedPages)
	}

	scanID := uuid.NewString()
	seq := 0
	for scanner.ScanPage() {
		seq++
		totalPages.Add(1)
		b, err := io.ReadAll(scanner.CurrentPage())
		if err != nil {
			close(pageChan)
			wg.Wait()
			return fmt.Errorf("unable to read page %d of scan %s: %w", seq, scanID, err)
		}
		pageChan <- models.ScannedPage{
			Reader:     bytes.NewReader(b),
			ScanID:     scanID,
			SequenceID: seq,
			ScanTime:   time.Now(),
		}
		time.Sleep(PageScanDelay) // Slow down infinite loops
	}

	if err := scanner.Err(); err != nil {
		close(pageChan)
		wg.Wait()
		return fmt.Errorf("scanner error for scan %s: %w", scanID, err)
	}

	close(pageChan)
	wg.Wait()

	if err := i.backend.Flush(ctx); err != nil {
		return fmt.Errorf("flush backend for scan %s: %w", scanID, err)
	}

	failed := failedPages.Load()
	total := totalPages.Load()
	if failed > 0 {
		log.Warnf("scan %s: %d of %d pages failed processing", scanID, failed, total)
	}
	return nil
}

func (i *Ingestor) ScanPagesWithDefaultWorkers(ctx context.Context, scanner DocumentsScanner) error {
	return i.ScanPages(ctx, scanner, DefaultWorkers)
}

// Ingest connects to the named AirScan scanner and streams every scanned
// page through the configured Backend.
func (i *Ingestor) Ingest(ctx context.Context, scannerName string, source string) error {
	c := airscan.NewClient(scannerName)
	settings := preset.GrayscaleA4ADF()
	settings.Duplex = false
	settings.ColorMode = "RGB24"
	settings.DocumentFormat = "image/jpeg"
	settings.InputSource = source

	job, err := c.Scan(settings)
	if err != nil {
		return fmt.Errorf("unable to create scan job on scanner %q source %q: %w", scannerName, source, err)
	}
	if err := i.ScanPages(ctx, job, DefaultWorkers); err != nil {
		return fmt.Errorf("scan pages from scanner %q source %q: %w", scannerName, source, err)
	}
	return nil
}

func (i *Ingestor) processPage(ctx context.Context, pageChan <-chan models.ScannedPage, wg *sync.WaitGroup, failedPages *atomic.Int64) {
	defer wg.Done()
	for page := range pageChan {
		if err := i.backend.ProcessPage(ctx, page); err != nil {
			log.Errorf("unable to process page scan=%s seq=%d: %v", page.ScanID, page.SequenceID, err)
			failedPages.Add(1)
		}
	}
}

// Ping checks the backend is reachable.
func (i *Ingestor) Ping(ctx context.Context) error {
	return i.backend.Ping(ctx)
}

// Close releases any resources held by the backend.
func (i *Ingestor) Close() error {
	return i.backend.Close()
}
