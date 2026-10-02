package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOnlyReadiness(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer upstream.Close()
			client, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{upstream.URL}}})
			require.NoError(t, err)
			server := &Server{osClient: client}
			report := server.readinessReport(context.Background())
			assert.Equal(t, status == http.StatusOK, report.Ready)
			assert.False(t, report.IngestionReady)
			require.Len(t, report.Checks, 1)
			assert.Equal(t, "opensearch", report.Checks[0].Name)
			if status != http.StatusOK {
				data, err := json.Marshal(report)
				require.NoError(t, err)
				assert.NotContains(t, string(data), upstream.URL)
				assert.Equal(t, "dependency unavailable", report.Checks[0].Detail)
			}
		})
	}
}
