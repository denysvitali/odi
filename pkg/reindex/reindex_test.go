package reindex

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
)

type mockStorage struct {
	pages map[string][]byte
}

func (m *mockStorage) Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error) {
	data, ok := m.pages[(models.ScannedPage{ScanID: scanID, SequenceID: sequenceNumber}).ID()]
	if !ok {
		return nil, model.ErrNotFound
	}
	return &models.ScannedPage{
		ScanID:     scanID,
		SequenceID: sequenceNumber,
		Reader:     bytes.NewReader(data),
	}, nil
}

type mockIndexer struct {
	duplicates map[string]string
	indexErrs  map[string]error
	released   []string
	indexed    []string
}

func (m *mockIndexer) ReserveContentDigest(ctx context.Context, digest string, documentID string) (indexer.ContentDigestReservation, error) {
	if existing := m.duplicates[documentID]; existing != "" {
		return indexer.ContentDigestReservation{ExistingDocumentID: existing}, nil
	}
	return indexer.ContentDigestReservation{Reserved: true}, nil
}

func (m *mockIndexer) ReleaseContentDigest(ctx context.Context, digest string, documentID string) error {
	m.released = append(m.released, documentID)
	return nil
}

func (m *mockIndexer) Index(ctx context.Context, page models.ScannedPage) error {
	if err := m.indexErrs[page.ID()]; err != nil {
		return err
	}
	m.indexed = append(m.indexed, page.ID())
	return nil
}

func TestRunTracksProcessedDuplicatesAndFailures(t *testing.T) {
	storage := &mockStorage{pages: map[string][]byte{
		"scan_1": []byte("one"),
		"scan_2": []byte("two"),
		"scan_3": []byte("three"),
	}}
	idx := &mockIndexer{
		duplicates: map[string]string{"scan_2": "other_1"},
		indexErrs:  map[string]error{"scan_3": errors.New("ocr failed")},
	}

	var events []PageResult
	result := Run(context.Background(), storage, idx, []models.ScannedPage{
		{ScanID: "scan", SequenceID: 1},
		{ScanID: "scan", SequenceID: 2},
		{ScanID: "scan", SequenceID: 3},
	}, func(pageResult PageResult, result Result) {
		events = append(events, pageResult)
	})

	assert.Equal(t, Result{Total: 3, Processed: 1, Duplicates: 1, Failed: 1}, result)
	assert.Equal(t, []string{"scan_1"}, idx.indexed)
	assert.Equal(t, []string{"scan_3"}, idx.released)
	require.Len(t, events, 3)
	assert.Equal(t, "indexed", events[0].Status)
	assert.Equal(t, "duplicate", events[1].Status)
	assert.Equal(t, "failed", events[2].Status)
}
