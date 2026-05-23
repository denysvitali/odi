package reindex

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

type Indexer interface {
	ReserveContentDigest(ctx context.Context, digest string, documentID string) (indexer.ContentDigestReservation, error)
	ReleaseContentDigest(ctx context.Context, digest string, documentID string) error
	Index(ctx context.Context, page models.ScannedPage) error
}

type PageResult struct {
	Page        models.ScannedPage
	Status      string
	DuplicateOf string
	Error       error
}

type Result struct {
	Total      int
	Processed  int
	Duplicates int
	Failed     int
}

type ProgressFunc func(PageResult, Result)

func Run(ctx context.Context, storage model.Retriever, idx Indexer, pages []models.ScannedPage, progress ProgressFunc) Result {
	result := Result{Total: len(pages)}
	for _, listedPage := range pages {
		if err := ctx.Err(); err != nil {
			result.Failed++
			emit(progress, PageResult{Page: listedPage, Status: "failed", Error: err}, result)
			return result
		}

		page, err := storage.Retrieve(ctx, listedPage.ScanID, listedPage.SequenceID)
		if err != nil {
			result.Failed++
			emit(progress, PageResult{Page: listedPage, Status: "failed", Error: fmt.Errorf("retrieve page: %w", err)}, result)
			continue
		}

		pageData, err := io.ReadAll(page.Reader)
		if err != nil {
			result.Failed++
			emit(progress, PageResult{Page: listedPage, Status: "failed", Error: fmt.Errorf("read page: %w", err)}, result)
			continue
		}

		page.Reader = bytes.NewReader(pageData)
		page.ContentDigest = contentdigest.Sum(pageData)

		reservation, err := idx.ReserveContentDigest(ctx, page.ContentDigest, page.ID())
		if err != nil {
			result.Failed++
			emit(progress, PageResult{Page: *page, Status: "failed", Error: fmt.Errorf("reserve content digest: %w", err)}, result)
			continue
		}
		if !reservation.Reserved && reservation.ExistingDocumentID != page.ID() {
			result.Duplicates++
			emit(progress, PageResult{Page: *page, Status: "duplicate", DuplicateOf: reservation.ExistingDocumentID}, result)
			continue
		}

		if err := idx.Index(ctx, *page); err != nil {
			if releaseErr := idx.ReleaseContentDigest(ctx, page.ContentDigest, page.ID()); releaseErr != nil {
				err = fmt.Errorf("%w; release content digest: %v", err, releaseErr)
			}
			result.Failed++
			emit(progress, PageResult{Page: *page, Status: "failed", Error: fmt.Errorf("index page: %w", err)}, result)
			continue
		}

		result.Processed++
		emit(progress, PageResult{Page: *page, Status: "indexed"}, result)
	}
	return result
}

func emit(progress ProgressFunc, pageResult PageResult, result Result) {
	if progress != nil {
		progress(pageResult, result)
	}
}
