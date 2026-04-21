package ingestor

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/denysvitali/odi/pkg/contentdigest"
	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
)

// Backend is the sink where scanned pages are sent for processing.
// Implementations may handle pages locally (store + OCR + index in-process)
// or remotely (POST to an odi server's upload endpoint).
type Backend interface {
	ProcessPage(ctx context.Context, page models.ScannedPage) error
	// Flush is called after a scan has finished so buffering backends can
	// commit their batch. Stateless backends may return nil.
	Flush(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error
}

// LocalBackend processes pages in-process: stores to the configured storage,
// then runs OCR + indexing via the local indexer.
type LocalBackend struct {
	idx     *indexer.Indexer
	storage model.Storer
}

func NewLocalBackend(config Config) (*LocalBackend, error) {
	var opts []indexer.Option
	if config.OpenSearchUsername != "" {
		opts = append(opts, indexer.WithOpenSearchUsername(config.OpenSearchUsername))
	}
	if config.OpenSearchPassword != "" {
		opts = append(opts, indexer.WithOpenSearchPassword(config.OpenSearchPassword))
	}
	if config.OpenSearchSkipTLS {
		opts = append(opts, indexer.WithOpenSearchSkipTLS())
	}
	opts = append(opts, indexer.WithOpenSearchIndex(config.OpenSearchIndex))
	idx, err := indexer.New(
		config.OpenSearchAddr, config.OcrAPIAddr, config.ZefixDsn,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create indexer: %w", err)
	}
	return &LocalBackend{idx: idx, storage: config.Storage}, nil
}

func (b *LocalBackend) ProcessPage(ctx context.Context, page models.ScannedPage) error {
	pageData, err := io.ReadAll(page.Reader)
	if err != nil {
		return fmt.Errorf("read page scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
	}

	page.ContentDigest = contentdigest.Sum(pageData)
	reservation, err := b.idx.ReserveContentDigest(ctx, page.ContentDigest, page.ID())
	if err != nil {
		return fmt.Errorf("reserve content digest scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
	}
	if !reservation.Reserved {
		log.Infof("scan=%s seq=%d duplicate of %s", page.ScanID, page.SequenceID, reservation.ExistingDocumentID)
		return nil
	}

	if b.storage != nil {
		err = b.storage.Store(ctx, models.ScannedPage{
			Reader:        bytes.NewReader(pageData),
			ScanID:        page.ScanID,
			SequenceID:    page.SequenceID,
			ContentDigest: page.ContentDigest,
		})
		if err != nil {
			if releaseErr := b.idx.ReleaseContentDigest(ctx, page.ContentDigest, page.ID()); releaseErr != nil {
				log.Warnf("scan=%s seq=%d: unable to release content digest after storage failure: %v", page.ScanID, page.SequenceID, releaseErr)
			}
			return fmt.Errorf("store page scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
		}
	}

	page.Reader = bytes.NewReader(pageData)
	if err := b.idx.Index(ctx, page); err != nil {
		if releaseErr := b.idx.ReleaseContentDigest(ctx, page.ContentDigest, page.ID()); releaseErr != nil {
			log.Warnf("scan=%s seq=%d: unable to release content digest after index failure: %v", page.ScanID, page.SequenceID, releaseErr)
		}
		return fmt.Errorf("index page scan=%s seq=%d: %w", page.ScanID, page.SequenceID, err)
	}
	return nil
}

func (b *LocalBackend) Flush(_ context.Context) error { return nil }

func (b *LocalBackend) Close() error { return nil }

func (b *LocalBackend) Ping(ctx context.Context) error {
	res, err := b.idx.PingOpensearch(ctx)
	if err != nil {
		return fmt.Errorf("unable to ping OpenSearch: %w", err)
	}
	if res.IsError() {
		return fmt.Errorf("unable to ping OpenSearch: %s", res.Status())
	}
	h, err := b.idx.PingOcrApi()
	if err != nil {
		return fmt.Errorf("unable to ping OCR API: %w", err)
	}
	if !h {
		return fmt.Errorf("OCR API is not healthy")
	}
	if err := b.idx.PingZefix(); err != nil {
		return fmt.Errorf("unable to ping Zefix: %w", err)
	}
	return nil
}
