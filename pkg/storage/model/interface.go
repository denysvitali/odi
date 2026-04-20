package model

import (
	"context"
	"io"

	"github.com/denysvitali/odi/pkg/models"
)

type Storer interface {
	Store(ctx context.Context, page models.ScannedPage) error
}

type Retriever interface {
	Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error)
}

type RWStorage interface {
	Storer
	Retriever
}

type ThumbnailStorer interface {
	StoreThumbnail(ctx context.Context, scanID string, sequenceNumber int, reader io.Reader) error
	ThumbnailExists(ctx context.Context, scanID string, sequenceNumber int) (bool, error)
}

type ThumbnailRetriever interface {
	RetrieveThumbnail(ctx context.Context, scanID string, sequenceNumber int) (*models.ThumbnailPage, error)
}

type ThumbnailStorage interface {
	ThumbnailStorer
	ThumbnailRetriever
}
