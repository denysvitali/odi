package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/contentdigest"
	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/storage/b2"
)

const (
	FlagScanID = "scan-id"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex [scan-id]",
	Short: "Re-index documents from B2 storage",
	Long: `Re-index previously stored documents from B2 storage.

This is useful for:
  - Applying new indexing rules to existing documents
  - Re-indexing documents that failed during initial ingestion
  - Updating document metadata with improved OCR or extraction`,
	Args: cobra.ExactArgs(1),
	RunE: runReindex,
}

func init() {
	AddStorageFlags(reindexCmd)
	AddOpenSearchFlags(reindexCmd)
	AddOCRFlags(reindexCmd)
	AddZefixFlags(reindexCmd)
}

func runReindex(cmd *cobra.Command, args []string) error {
	if GetString(cmd, FlagOsAddr) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OPENSEARCH_ADDR)", FlagOsAddr)
	}
	if GetString(cmd, FlagOcrAPIAddr) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OCR_API_ADDR)", FlagOcrAPIAddr)
	}
	if GetString(cmd, FlagZefixDsn) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: ZEFIX_DSN)", FlagZefixDsn)
	}

	log := logrus.StandardLogger()
	scanID := args[0]

	ui.PrintInfof("Re-indexing scan: %s", scanID)

	store, err := GetStorage(cmd)
	if err != nil {
		ui.PrintErrorf("Failed to initialize storage: %v", err)
		return err
	}
	b, ok := store.(*b2.B2)
	if !ok {
		return fmt.Errorf("reindex requires B2 storage (got %T)", store)
	}

	var opts []indexer.Option
	if username := GetString(cmd, FlagOsUsername); username != "" {
		opts = append(opts, indexer.WithOpenSearchUsername(username))
	}
	if password := GetString(cmd, FlagOsPassword); password != "" {
		opts = append(opts, indexer.WithOpenSearchPassword(password))
	}
	if GetBool(cmd, FlagOsSkipTLS) {
		opts = append(opts, indexer.WithOpenSearchSkipTLS())
	}
	opts = append(opts, indexer.WithOpenSearchIndex(GetString(cmd, FlagOsIndex)))

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

	scanFiles, err := b.ListFiles(scanID)
	if err != nil {
		ui.PrintErrorf("Failed to list files: %v", err)
		return err
	}

	ui.PrintInfof("Found %d files to re-index", len(scanFiles))

	ctx := context.Background()
	for _, f := range scanFiles {
		log.Infof("Indexing %s", f.ID())
		scannedPage, err := b.Retrieve(ctx, f.ScanID, f.SequenceID)
		if err != nil {
			log.Errorf("Failed to retrieve file %s: %v", f.ID(), err)
			continue
		}
		pageData, err := io.ReadAll(scannedPage.Reader)
		if err != nil {
			log.Errorf("Failed to read file %s: %v", f.ID(), err)
			continue
		}
		scannedPage.Reader = bytes.NewReader(pageData)
		scannedPage.ContentDigest = contentdigest.Sum(pageData)

		reservation, err := idx.ReserveContentDigest(ctx, scannedPage.ContentDigest, scannedPage.ID())
		if err != nil {
			log.Errorf("Failed to reserve content digest for %s: %v", f.ID(), err)
			continue
		}
		if !reservation.Reserved && reservation.ExistingDocumentID != scannedPage.ID() {
			log.Infof("Skipping duplicate file %s; duplicate of %s", f.ID(), reservation.ExistingDocumentID)
			continue
		}
		err = idx.Index(ctx, *scannedPage)
		if err != nil {
			if releaseErr := idx.ReleaseContentDigest(ctx, scannedPage.ContentDigest, scannedPage.ID()); releaseErr != nil {
				log.Warnf("Failed to release content digest for %s after index failure: %v", f.ID(), releaseErr)
			}
			log.Errorf("Failed to index file %s: %v", f.ID(), err)
			continue
		}
	}

	ui.PrintSuccess("Re-indexing complete")
	return nil
}
