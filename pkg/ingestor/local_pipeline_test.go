package ingestor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/denysvitali/odi/pkg/contentdigest"
	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/stretchr/testify/require"
)

type pipelineIndexer struct {
	reserve func(context.Context, string, string) (indexer.ContentDigestReservation, error)
	release func(context.Context, string, string) error
	index   func(context.Context, models.ScannedPage) error
}

func (i pipelineIndexer) ReserveContentDigest(ctx context.Context, digest, id string) (indexer.ContentDigestReservation, error) {
	return i.reserve(ctx, digest, id)
}

func (i pipelineIndexer) ReleaseContentDigest(ctx context.Context, digest, id string) error {
	return i.release(ctx, digest, id)
}

func (i pipelineIndexer) Index(ctx context.Context, page models.ScannedPage) error {
	return i.index(ctx, page)
}

type pipelineStorage func(context.Context, models.ScannedPage) error

func (s pipelineStorage) Store(ctx context.Context, page models.ScannedPage) error {
	return s(ctx, page)
}

func TestLocalPipelineStages(t *testing.T) {
	stageErr := errors.New("synthetic stage failure")
	for _, tc := range []struct {
		name       string
		failure    string
		duplicate  bool
		noStorage  bool
		cancel     bool
		wantEvents []string
	}{
		{name: "success", wantEvents: []string{"reserve", "store", "index"}},
		{name: "without storage", noStorage: true, wantEvents: []string{"reserve", "index"}},
		{name: "duplicate", duplicate: true, wantEvents: []string{"reserve"}},
		{name: "reserve failure", failure: "reserve", wantEvents: []string{"reserve"}},
		{name: "storage failure", failure: "store", wantEvents: []string{"reserve", "store", "release"}},
		{name: "index failure retains blob", failure: "index", wantEvents: []string{"reserve", "store", "index", "release"}},
		{name: "index failure without storage", failure: "index", noStorage: true, wantEvents: []string{"reserve", "index", "release"}},
		{name: "canceled storage cleans up", failure: "store", cancel: true, wantEvents: []string{"reserve", "store", "release"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type contextKey struct{}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "marker"))
			defer cancel()
			payload := []byte("synthetic page")
			digest := contentdigest.Sum(payload)
			scanTime := time.Now()
			page := models.ScannedPage{
				Reader: bytes.NewReader(payload), ScanID: "scan-test", SequenceID: 4,
				ScanTime: scanTime, ContentDigest: "untrusted incoming digest",
			}
			var events []string
			idx := pipelineIndexer{
				reserve: func(callCtx context.Context, gotDigest, id string) (indexer.ContentDigestReservation, error) {
					events = append(events, "reserve")
					require.Equal(t, ctx, callCtx)
					require.Equal(t, digest, gotDigest)
					require.Equal(t, page.ID(), id)
					if tc.failure == "reserve" {
						return indexer.ContentDigestReservation{}, stageErr
					}
					return indexer.ContentDigestReservation{Reserved: !tc.duplicate, ExistingDocumentID: "earlier_1"}, nil
				},
				release: func(cleanupCtx context.Context, gotDigest, id string) error {
					events = append(events, "release")
					require.NoError(t, cleanupCtx.Err())
					require.Equal(t, "marker", cleanupCtx.Value(contextKey{}))
					deadline, ok := cleanupCtx.Deadline()
					require.True(t, ok)
					require.WithinDuration(t, time.Now().Add(10*time.Second), deadline, time.Second)
					require.Equal(t, digest, gotDigest)
					require.Equal(t, page.ID(), id)
					// Cleanup failure must not replace the actionable stage error.
					return errors.New("synthetic cleanup failure")
				},
				index: func(callCtx context.Context, got models.ScannedPage) error {
					events = append(events, "index")
					require.Equal(t, ctx, callCtx)
					require.Equal(t, scanTime, got.ScanTime)
					require.Equal(t, digest, got.ContentDigest)
					data, err := io.ReadAll(got.Reader)
					require.NoError(t, err)
					require.Equal(t, payload, data, "index must receive a fresh reader after storage consumed it")
					if tc.failure == "index" {
						return stageErr
					}
					return nil
				},
			}
			var storage pipelineStorage
			if !tc.noStorage {
				storage = func(callCtx context.Context, got models.ScannedPage) error {
					events = append(events, "store")
					require.Equal(t, ctx, callCtx)
					require.Equal(t, page.ID(), got.ID())
					require.Equal(t, digest, got.ContentDigest)
					data, err := io.ReadAll(got.Reader)
					require.NoError(t, err)
					require.Equal(t, payload, data)
					if tc.cancel {
						cancel()
					}
					if tc.failure == "store" {
						return stageErr
					}
					return nil
				}
			}
			// A nil concrete function converted to Storer is non-nil; pass an
			// actual nil interface for the optional-storage path.
			var err error
			if tc.noStorage {
				err = processLocalPage(ctx, idx, nil, page)
			} else {
				err = processLocalPage(ctx, idx, storage, page)
			}
			if tc.failure != "" {
				require.ErrorIs(t, err, stageErr)
				require.ErrorContains(t, err, tc.failure)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantEvents, events)
		})
	}
}

type failingPageReader struct{ err error }

func (r failingPageReader) Read([]byte) (int, error)       { return 0, r.err }
func (r failingPageReader) Seek(int64, int) (int64, error) { return 0, r.err }

func TestLocalPipelineReadFailureDoesNotReserve(t *testing.T) {
	readErr := errors.New("synthetic reader failure")
	idx := pipelineIndexer{reserve: func(context.Context, string, string) (indexer.ContentDigestReservation, error) {
		t.Fatal("failed page reads must not reserve a digest")
		return indexer.ContentDigestReservation{}, nil
	}}
	err := processLocalPage(context.Background(), idx, nil, models.ScannedPage{
		Reader: failingPageReader{err: readErr}, ScanID: "scan-test", SequenceID: 1,
	})
	require.ErrorIs(t, err, readErr)
}
