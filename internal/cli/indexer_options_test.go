package cli

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandIndexerOptionsPreserveConfigurationPrecedence(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "environment"
		if explicit {
			name = "explicit flags"
		}
		t.Run(name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			cmd := &cobra.Command{}
			AddOpenSearchFlags(cmd)
			t.Setenv("OPENSEARCH_USERNAME", "environment-user")
			t.Setenv("OPENSEARCH_PASSWORD", "synthetic-environment-password")
			t.Setenv("OPENSEARCH_INDEX", "environment-documents")
			t.Setenv("OPENSEARCH_SKIP_TLS", "true")

			user, password, index := "environment-user", "synthetic-environment-password", "environment-documents"
			if explicit {
				user, password, index = "flag-user", "synthetic-flag-password", "flag-documents"
				require.NoError(t, cmd.Flags().Set(FlagOsUsername, user))
				require.NoError(t, cmd.Flags().Set(FlagOsPassword, password))
				require.NoError(t, cmd.Flags().Set(FlagOsIndex, index))
			}

			paths := make(chan string, 4)
			search := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUser, gotPassword, ok := r.BasicAuth()
				assert.True(t, ok)
				assert.Equal(t, user, gotUser)
				assert.Equal(t, password, gotPassword)
				paths <- r.URL.Path
				w.WriteHeader(http.StatusOK)
			}))
			defer search.Close()
			search.Config.ErrorLog = log.New(io.Discard, "", 0)
			ocr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/healthz", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			}))
			defer ocr.Close()
			t.Setenv("OCR_ALLOW_PRIVATE_TARGETS", "true")
			t.Setenv("ODI_OCR_ALLOW_PRIVATE_TARGETS", "true")
			t.Setenv("OCR_ALLOWED_HOSTS", "127.0.0.1")
			t.Setenv("ODI_OCR_ALLOWED_HOSTS", "127.0.0.1")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := indexer.NewWithContext(ctx, search.URL, ocr.URL, "", commandIndexerOptions(cmd, nil)...)
			require.NoError(t, err)
			assert.Equal(t, "/"+index, <-paths)
			assert.Equal(t, "/"+index+"_digests", <-paths)

			// Explicit false must override the environment opt-in and restore
			// certificate verification against this self-signed test server.
			require.NoError(t, cmd.Flags().Set(FlagOsSkipTLS, "false"))
			_, err = indexer.NewWithContext(ctx, search.URL, ocr.URL, "", commandIndexerOptions(cmd, nil)...)
			require.ErrorContains(t, err, "certificate")
		})
	}
}
