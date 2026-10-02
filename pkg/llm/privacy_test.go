package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestResponseBoundsAndPrivacy(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"valid", http.StatusOK, `{"message":{"content":"synthetic answer"}}`, ""},
		{"upstream failure", http.StatusServiceUnavailable, "private-response-marker", "unexpected LLM status code 503"},
		{"over limit", http.StatusOK, strings.Repeat("x", maxResponseBytes+1), "LLM response exceeds size limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()
			c, err := New(srv.URL)
			require.NoError(t, err)
			body, err := c.doRequest(context.Background(), c.chatURL(), []byte(`{}`))
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				require.Nil(t, body)
				require.NotContains(t, err.Error(), "private-response-marker")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.body, string(body))
			}
		})
	}
}

func TestRequestCancellation(t *testing.T) {
	c, err := New("http://127.0.0.1:1")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.doRequest(ctx, c.chatURL(), []byte(`{}`))
	require.ErrorIs(t, err, context.Canceled)
}
