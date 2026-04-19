package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/models"
)

var pdfCmd = &cobra.Command{
	Use:   "pdf [input-folder]",
	Short: "Index PDF documents from a directory",
	Long: `Index PDF documents from a local directory.

This command will:
  1. Walk the specified directory for .pdf files
  2. Extract embedded page images from each PDF using pdfcpu
  3. Process each image through the OCR pipeline
  4. Index the extracted text and metadata in OpenSearch`,
	Args: cobra.ExactArgs(1),
	RunE: runPDF,
}

func init() {
	pdfCmd.Flags().IntP(FlagWorkers, "w", DefaultIndexWorkers, "Number of worker goroutines")

	AddOpenSearchFlags(pdfCmd)
	AddOCRFlags(pdfCmd)
	AddZefixFlags(pdfCmd)
}

func runPDF(cmd *cobra.Command, args []string) error {
	if GetString(cmd, FlagOsAddr) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OPENSEARCH_ADDR)", FlagOsAddr)
	}
	if GetString(cmd, FlagOsUsername) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OPENSEARCH_USERNAME)", FlagOsUsername)
	}
	if GetString(cmd, FlagOsPassword) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OPENSEARCH_PASSWORD)", FlagOsPassword)
	}
	if GetString(cmd, FlagOcrAPIAddr) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OCR_API_ADDR)", FlagOcrAPIAddr)
	}
	if GetString(cmd, FlagZefixDsn) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: ZEFIX_DSN)", FlagZefixDsn)
	}

	log := logrus.StandardLogger()
	inputDir := args[0]

	opts := []indexer.Option{
		indexer.WithOpenSearchUsername(GetString(cmd, FlagOsUsername)),
		indexer.WithOpenSearchPassword(GetString(cmd, FlagOsPassword)),
		indexer.WithOcrApiCAPath(GetString(cmd, FlagOcrCaPath)),
	}
	if GetBool(cmd, FlagOsSkipTLS) {
		opts = append(opts, indexer.WithOpenSearchSkipTLS())
	}

	idx, err := indexer.New(
		GetString(cmd, FlagOsAddr),
		GetString(cmd, FlagOcrAPIAddr),
		GetString(cmd, FlagZefixDsn),
		opts...,
	)
	if err != nil {
		ui.PrintErrorf("Failed to create indexer: %v", err)
		return err
	}

	ctx := context.Background()
	res, err := idx.PingOpensearch(ctx)
	if err != nil {
		ui.PrintErrorf("Failed to ping OpenSearch: %v", err)
		return err
	}
	if res.IsError() {
		ui.PrintErrorf("OpenSearch ping failed: %s", res.Status())
		return nil
	}
	ui.PrintSuccess("Connected to OpenSearch")

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		ui.PrintErrorf("Failed to read directory: %v", err)
		return err
	}

	var pdfFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			pdfFiles = append(pdfFiles, filepath.Join(inputDir, entry.Name()))
		}
	}

	if len(pdfFiles) == 0 {
		ui.PrintWarningf("No PDF files found in %s", inputDir)
		return nil
	}

	ui.PrintInfof("Found %d PDF file(s) in %s", len(pdfFiles), inputDir)

	scanID := uuid.NewString()
	seq := 0
	indexed := 0

	for _, pdfPath := range pdfFiles {
		log.Infof("Processing %s", filepath.Base(pdfPath))
		n, err := indexPDF(ctx, idx, pdfPath, scanID, &seq)
		if err != nil {
			log.Errorf("Failed to process %s: %v", pdfPath, err)
			continue
		}
		indexed += n
	}

	ui.PrintSuccessf("Indexing complete. %d image(s) indexed from %d PDF(s). Scan ID: %s",
		indexed, len(pdfFiles), scanID)
	return nil
}

// indexPDF extracts embedded images from a PDF and indexes each one.
// It returns the number of images successfully submitted for indexing.
func indexPDF(ctx context.Context, idx *indexer.Indexer, pdfPath, scanID string, seq *int) (int, error) {
	f, err := os.Open(pdfPath)
	if err != nil {
		return 0, fmt.Errorf("open PDF: %w", err)
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	// ExtractImagesRaw returns one map per page; each map key is the object number.
	pageImages, err := api.ExtractImagesRaw(f, nil, conf)
	if err != nil {
		return 0, fmt.Errorf("extract images from PDF: %w", err)
	}

	count := 0
	for _, pageMap := range pageImages {
		for _, img := range pageMap {
			data, readErr := io.ReadAll(img.Reader)
			if readErr != nil {
				logrus.Errorf("Failed to read image from %s: %v", filepath.Base(pdfPath), readErr)
				continue
			}

			*seq++
			indexErr := idx.Index(ctx, models.ScannedPage{
				Reader:     bytes.NewReader(data),
				ScanID:     scanID,
				SequenceID: *seq,
			})
			if indexErr != nil {
				logrus.Errorf("Failed to index image (obj %d) from %s: %v",
					img.ObjNr, filepath.Base(pdfPath), indexErr)
				continue
			}
			count++
		}
	}

	if count == 0 && len(pageImages) == 0 {
		logrus.Warnf("No embedded images found in %s (text-only PDF?)", filepath.Base(pdfPath))
	}

	return count, nil
}
