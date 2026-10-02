package ingestor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScannerHTTPClientCancelsResponseRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	response, err := (scannerHTTPClient{ctx: ctx}).Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	result := make(chan error, 1)
	go func() { _, err := io.ReadAll(response.Body); result <- err }()
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("scanner body read ignored cancellation")
	}
}
