package model

import (
	"context"
	"errors"
	"io"

	"github.com/denysvitali/odi/pkg/models"
)

// ErrNotFound is returned by storage backends when an object does not exist.
var ErrNotFound = errors.New("object not found")

type Storer interface {
	Store(ctx context.Context, page models.ScannedPage) error
}

type Retriever interface {
	Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error)
}

type Deleter interface {
	Delete(ctx context.Context, scanID string, sequenceNumber int) error
}

type PageLister interface {
	ListPages(ctx context.Context) ([]models.ScannedPage, error)
}

type RWStorage interface {
	Storer
	Retriever
	Deleter
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
