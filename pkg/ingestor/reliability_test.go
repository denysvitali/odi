package ingestor_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/denysvitali/odi/pkg/ingestor"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/stretchr/testify/require"
)

type reliabilityBackend struct {
	process func(context.Context, models.ScannedPage) error
	flushed atomic.Int32
}

func (b *reliabilityBackend) ProcessPage(ctx context.Context, p models.ScannedPage) error {
	return b.process(ctx, p)
}
func (b *reliabilityBackend) Flush(context.Context) error { b.flushed.Add(1); return nil }
func (*reliabilityBackend) Ping(context.Context) error    { return nil }
func (*reliabilityBackend) Close() error                  { return nil }

func TestScanPagesReportsFailuresAndContinuesAfterPanic(t *testing.T) {
	for _, panicPage := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panicPage], func(t *testing.T) {
			var processed atomic.Int32
			backend := &reliabilityBackend{process: func(_ context.Context, p models.ScannedPage) error {
				processed.Add(1)
				if p.SequenceID == 1 {
					if panicPage {
						panic("synthetic")
					}
					return errors.New("synthetic failure")
				}
				return nil
			}}
			scanner := &testScanner{files: []io.Reader{bytes.NewReader([]byte("one")), bytes.NewReader([]byte("two"))}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := ingestor.NewWithBackend(backend).ScanPages(ctx, scanner, 1)
			require.ErrorContains(t, err, "1 of 2 pages failed processing")
			require.Equal(t, int32(2), processed.Load())
			require.Equal(t, int32(1), backend.flushed.Load())
		})
	}
}

type observedScanner struct {
	testScanner
	thirdPage chan struct{}
}

func (s *observedScanner) ScanPage() bool {
	if !s.testScanner.ScanPage() {
		return false
	}
	if s.idx == 3 {
		close(s.thirdPage)
	}
	return true
}

func TestScanPagesCancellationInterruptsBlockedProducer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	backend := &reliabilityBackend{process: func(ctx context.Context, _ models.ScannedPage) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	scanner := &observedScanner{testScanner: testScanner{files: []io.Reader{bytes.NewReader([]byte("one")), bytes.NewReader([]byte("two")), bytes.NewReader([]byte("three"))}}, thirdPage: make(chan struct{})}
	result := make(chan error, 1)
	go func() { result <- ingestor.NewWithBackend(backend).ScanPages(ctx, scanner, 1) }()
	<-started
	select {
	case <-scanner.thirdPage:
	case <-time.After(time.Second):
		t.Fatal("producer did not reach full queue")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("scan did not stop after cancellation")
	}
}

type failingScanner struct{ testScanner }

func (*failingScanner) Err() error { return errors.New("scanner disconnected") }
func TestScanPagesFlushesAcceptedPagesAfterScannerFailure(t *testing.T) {
	backend := &reliabilityBackend{process: func(context.Context, models.ScannedPage) error { return nil }}
	scanner := &failingScanner{testScanner{files: []io.Reader{bytes.NewReader([]byte("one"))}}}
	err := ingestor.NewWithBackend(backend).ScanPages(context.Background(), scanner, 1)
	require.ErrorContains(t, err, "scanner disconnected")
	require.Equal(t, int32(1), backend.flushed.Load())
}
