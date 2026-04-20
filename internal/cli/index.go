package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/ingestor"
	"github.com/denysvitali/odi/pkg/models"
)

const (
	FlagInputDir = "input-dir"
)

var indexCmd = &cobra.Command{
	Use:   "index [input-dir]",
	Short: "Index documents from a directory",
	Long: `Index documents from a local directory.

This command will:
  1. Read all image files from the specified directory
  2. Process them through OCR (or POST to a remote backend)
  3. Extract text and metadata (companies, dates, barcodes)
  4. Store the indexed documents in OpenSearch

Backend targets:
  - local (default):  in-process OCR + OpenSearch indexing.
  - remote:           POST files to an odi server's /api/v1/upload endpoint.`,
	Args: cobra.ExactArgs(1),
	RunE: runIndex,
}

func init() {
	indexCmd.Flags().IntP(FlagWorkers, "w", DefaultIndexWorkers, "Number of worker goroutines")
	indexCmd.Flags().BoolP(FlagDebug, "D", false, "Enable debug logging")
	indexCmd.Flags().String(FlagBackend, BackendLocal, "Backend target: local or remote (env: BACKEND)")
	indexCmd.Flags().String(FlagBackendURL, "", "Remote backend base URL (env: BACKEND_URL)")
	indexCmd.Flags().String(FlagBackendToken, "", "Bearer token for the remote backend (env: BACKEND_TOKEN)")

	bindEnv(FlagBackend, "BACKEND")
	bindEnv(FlagBackendURL, "BACKEND_URL")
	bindEnv(FlagBackendToken, "BACKEND_TOKEN")

	AddOpenSearchFlags(indexCmd)
	AddOCRFlags(indexCmd)
	AddZefixFlags(indexCmd)
}

func runIndex(cmd *cobra.Command, args []string) error {
	log := logrus.StandardLogger()
	inputDir := args[0]

	debug := GetBool(cmd, FlagDebug)
	if debug {
		log.SetLevel(logrus.DebugLevel)
	}

	workers := GetInt(cmd, FlagWorkers)
	if workers <= 0 {
		workers = DefaultIndexWorkers
		ui.PrintWarningf("Workers cannot be <= 0, using default: %d", workers)
	}

	backend, err := buildIndexBackend(cmd)
	if err != nil {
		ui.PrintErrorf("Failed to initialize backend: %v", err)
		return err
	}
	defer backend.Close()

	ctx := context.Background()

	if err := backend.Ping(ctx); err != nil {
		ui.PrintErrorf("Backend not reachable: %v", err)
		return err
	}

	files, err := os.ReadDir(inputDir)
	if err != nil {
		ui.PrintErrorf("Failed to read directory: %v", err)
		return err
	}

	// Filter to image-like files only
	var imageFiles []os.DirEntry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name()))
		for _, supported := range []string{".jpg", ".jpeg", ".png", ".tiff", ".tif", ".bmp", ".gif", ".webp", ".pdf"} {
			if ext == supported {
				imageFiles = append(imageFiles, f)
				break
			}
		}
	}

	ui.PrintInfof("Processing %d files from %s", len(imageFiles), inputDir)

	ch := make(chan models.ScannedPage, workers)
	wg := sync.WaitGroup{}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for page := range ch {
				if err := backend.ProcessPage(ctx, page); err != nil {
					log.Errorf("unable to process %s seq=%d: %v", page.ScanID, page.SequenceID, err)
				}
			}
		}()
	}

	scanID := uuid.NewString()
	seq := 0
	for _, file := range imageFiles {
		seq++
		data, err := os.ReadFile(path.Join(inputDir, file.Name()))
		if err != nil {
			log.Errorf("Unable to read file %s: %v", file.Name(), err)
			continue
		}
		ch <- models.ScannedPage{
			Reader:     bytes.NewReader(data),
			ScanID:     scanID,
			SequenceID: seq,
		}
	}
	close(ch)
	wg.Wait()

	if err := backend.Flush(ctx); err != nil {
		ui.PrintErrorf("Flush failed: %v", err)
		return err
	}

	ui.PrintSuccessf("Indexing complete. Scan ID: %s", scanID)
	return nil
}

func buildIndexBackend(cmd *cobra.Command) (ingestor.Backend, error) {
	kind := resolveBackendKind(cmd)
	switch kind {
	case BackendLocal:
		return buildLocalIndexBackend(cmd)
	case BackendRemote:
		return buildRemoteBackend(cmd)
	default:
		return nil, fmt.Errorf("unknown backend %q (expected %q or %q)", kind, BackendLocal, BackendRemote)
	}
}

func buildLocalIndexBackend(cmd *cobra.Command) (ingestor.Backend, error) {
	if err := RequireFlags(cmd, FlagOsAddr, FlagOcrAPIAddr, FlagZefixDsn); err != nil {
		return nil, fmt.Errorf("local backend: %w", err)
	}

	return ingestor.NewLocalBackend(ingestor.Config{
		OpenSearchAddr:     GetString(cmd, FlagOsAddr),
		OpenSearchUsername: GetString(cmd, FlagOsUsername),
		OpenSearchPassword: GetString(cmd, FlagOsPassword),
		OpenSearchSkipTLS:  GetBool(cmd, FlagOsSkipTLS),
		OcrAPIAddr:         GetString(cmd, FlagOcrAPIAddr),
		ZefixDsn:           GetString(cmd, FlagZefixDsn),
		Storage:            nil,
	})
}
