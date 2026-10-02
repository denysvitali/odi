package ingestor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/denysvitali/odi/pkg/contentdigest"
	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
)

// pageIndexer is the local pipeline's indexing contract. Health checks and
// indexer construction remain the responsibility of LocalBackend.
type pageIndexer interface {
	ReserveContentDigest(context.Context, string, string) (indexer.ContentDigestReservation, error)
	ReleaseContentDigest(context.Context, string, string) error
	Index(context.Context, models.ScannedPage) error
}

// processLocalPage reserves the digest before performing any durable page work.
// A successful index keeps the reservation; a failed storage or indexing stage
// releases it so a later attempt can retry. Stored blobs survive index failures
// because the index request may have committed before its response was lost.
func processLocalPage(ctx context.Context, idx pageIndexer, storage model.Storer, page models.ScannedPage) (err error) {
	pageData, err := io.ReadAll(page.Reader)
	if err != nil {
		return fmt.Errorf("read page scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
	}

	page.ContentDigest = contentdigest.Sum(pageData)
	reservation, err := idx.ReserveContentDigest(ctx, page.ContentDigest, page.ID())
	if err != nil {
		return fmt.Errorf("reserve content digest scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
	}
	if !reservation.Reserved {
		log.Infof("scan=%s seq=%d duplicate of %s", page.ScanID, page.SequenceID, reservation.ExistingDocumentID)
		return nil
	}

	// Only release a reservation we own, and keep the original stage error even
	// if cleanup fails. Detached cleanup still has a bounded lifetime.
	defer func() {
		if err == nil {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if releaseErr := idx.ReleaseContentDigest(cleanupCtx, page.ContentDigest, page.ID()); releaseErr != nil {
			log.WithFields(logrus.Fields{
				"event":      "digest_release_failed",
				"scanID":     page.ScanID,
				"sequenceID": page.SequenceID,
			}).Error("unable to release content digest after page processing failure")
		}
	}()

	if storage != nil {
		storedPage := models.ScannedPage{
			Reader:        bytes.NewReader(pageData),
			ScanID:        page.ScanID,
			SequenceID:    page.SequenceID,
			ContentDigest: page.ContentDigest,
		}
		if err := storage.Store(ctx, storedPage); err != nil {
			return fmt.Errorf("store page scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
		}
	}

	page.Reader = bytes.NewReader(pageData)
	if err := idx.Index(ctx, page); err != nil {
		if storage != nil {
			log.WithFields(logrus.Fields{
				"event":      "orphan_blob",
				"scanID":     page.ScanID,
				"sequenceID": page.SequenceID,
				"digest":     page.ContentDigest,
			}).Error("indexing failed; blob retained for reindex recovery")
		}
		return fmt.Errorf("index page scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
	}
	return nil
}
