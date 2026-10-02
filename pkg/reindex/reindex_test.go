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

type trackedReader struct {
	*bytes.Reader
	closed   bool
	readErr  error
	closeErr error
}

func (r *trackedReader) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return r.Reader.Read(p)
}
func (r *trackedReader) Close() error { r.closed = true; return r.closeErr }

type readerStorage struct{ reader *trackedReader }

func (s readerStorage) Retrieve(context.Context, string, int) (*models.ScannedPage, error) {
	return &models.ScannedPage{ScanID: "scan", SequenceID: 1, Reader: s.reader}, nil
}
func TestRunClosesRetrievedReaderOnAllReadPaths(t *testing.T) {
	for _, kind := range []string{"success", "read failure", "close failure"} {
		t.Run(kind, func(t *testing.T) {
			reader := &trackedReader{Reader: bytes.NewReader([]byte("synthetic"))}
			if kind == "read failure" {
				reader.readErr = errors.New("read failed")
			}
			if kind == "close failure" {
				reader.closeErr = errors.New("close failed")
			}
			idx := &mockIndexer{}
			result := Run(context.Background(), readerStorage{reader}, idx, []models.ScannedPage{{ScanID: "scan", SequenceID: 1}}, nil)
			require.True(t, reader.closed)
			if kind == "success" {
				require.Equal(t, 1, result.Processed)
			} else {
				require.Equal(t, 1, result.Failed)
				require.Empty(t, idx.indexed)
			}
		})
	}
}
func TestRunReindexesBlobWithExistingOwnedReservation(t *testing.T) {
	idx := &mockIndexer{duplicates: map[string]string{"scan_1": "scan_1"}}
	result := Run(context.Background(), &mockStorage{pages: map[string][]byte{"scan_1": []byte("synthetic")}}, idx, []models.ScannedPage{{ScanID: "scan", SequenceID: 1}}, nil)
	require.Equal(t, Result{Total: 1, Processed: 1}, result)
	require.Equal(t, []string{"scan_1"}, idx.indexed)
}

type cancelingIndexer struct {
	mockIndexer
	cancel          context.CancelFunc
	cleanupErr      error
	cleanupDeadline bool
}

func (i *cancelingIndexer) Index(ctx context.Context, p models.ScannedPage) error {
	i.cancel()
	return ctx.Err()
}
func (i *cancelingIndexer) ReleaseContentDigest(ctx context.Context, digest, documentID string) error {
	i.cleanupErr = ctx.Err()
	_, i.cleanupDeadline = ctx.Deadline()
	return i.mockIndexer.ReleaseContentDigest(ctx, digest, documentID)
}
func TestRunCleansReservationAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	idx := &cancelingIndexer{cancel: cancel}
	result := Run(ctx, &mockStorage{pages: map[string][]byte{"scan_1": []byte("synthetic")}}, idx, []models.ScannedPage{{ScanID: "scan", SequenceID: 1}}, nil)
	require.Equal(t, 1, result.Failed)
	require.NoError(t, idx.cleanupErr)
	require.True(t, idx.cleanupDeadline)
	require.Equal(t, []string{"scan_1"}, idx.released)
}
