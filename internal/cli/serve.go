package cli

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denysvitali/odi/internal/server"
	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/indexer"
)

const (
	FlagAllowUnauthenticatedPublic = "allow-unauthenticated-public"
	FlagListenAddr                 = "listen-addr"
	FlagAPIToken                   = "api-token"
	FlagTLSCertPath                = "tls-cert-path"
	FlagTLSKeyPath                 = "tls-key-path"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the REST API server",
	Long: `Start the ODI REST API server for searching, retrieving, and uploading documents.

The server provides the following endpoints:
  - POST /api/v1/search      - Search documents
  - GET  /api/v1/documents   - List documents
  - GET  /api/v1/documents/:id - Get document by ID
  - GET  /api/v1/files/:scanID/:sequenceId - Get document file
  - POST /api/v1/upload      - Upload and index JPG files`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringP(FlagListenAddr, "L", "127.0.0.1:8085", "Address to listen on")
	_ = viper.BindPFlag(FlagListenAddr, serveCmd.Flags().Lookup(FlagListenAddr))

	serveCmd.Flags().String(FlagAPIToken, "", "Bearer token required on /api/v1 routes; if empty, authentication is disabled (env: API_TOKEN)")
	_ = viper.BindPFlag(FlagAPIToken, serveCmd.Flags().Lookup(FlagAPIToken))
	bindEnv(FlagAPIToken, "API_TOKEN")

	serveCmd.Flags().String(FlagTLSCertPath, "", "Path to TLS certificate file; if both cert and key are set, the server uses HTTPS (env: TLS_CERT_PATH)")
	_ = viper.BindPFlag(FlagTLSCertPath, serveCmd.Flags().Lookup(FlagTLSCertPath))
	bindEnv(FlagTLSCertPath, "TLS_CERT_PATH")

	serveCmd.Flags().String(FlagTLSKeyPath, "", "Path to TLS key file; if both cert and key are set, the server uses HTTPS (env: TLS_KEY_PATH)")
	_ = viper.BindPFlag(FlagTLSKeyPath, serveCmd.Flags().Lookup(FlagTLSKeyPath))
	bindEnv(FlagTLSKeyPath, "TLS_KEY_PATH")

	serveCmd.Flags().Bool(FlagAllowUnauthenticatedPublic, false, "Allow a non-loopback listener without API_TOKEN (env: ALLOW_UNAUTHENTICATED_PUBLIC)")
	_ = viper.BindPFlag(FlagAllowUnauthenticatedPublic, serveCmd.Flags().Lookup(FlagAllowUnauthenticatedPublic))
	bindEnv(FlagAllowUnauthenticatedPublic, "ALLOW_UNAUTHENTICATED_PUBLIC")

	AddOpenSearchFlags(serveCmd)
	AddStorageFlags(serveCmd)
	AddOCRFlags(serveCmd)
	AddZefixFlags(serveCmd)
	AddLLMFlags(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	if err := validateServerListener(GetString(cmd, FlagListenAddr), GetString(cmd, FlagAPIToken), GetBool(cmd, FlagAllowUnauthenticatedPublic)); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if GetString(cmd, FlagOsAddr) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: OPENSEARCH_ADDR)", FlagOsAddr)
	}
	if GetString(cmd, FlagStorageType) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: STORAGE_TYPE)", FlagStorageType)
	}

	storage, err := GetStorage(cmd)
	if err != nil {
		ui.PrintErrorf("Failed to initialize storage: %v", err)
		return err
	}
	if storage == nil {
		ui.PrintError("Invalid storage type. Must be 'b2' or 'fs'")
		return fmt.Errorf("storage backend is unavailable")
	}

	llmClient, err := BuildLLMClient(cmd)
	if err != nil {
		return fmt.Errorf("configure LLM: %w", err)
	}
	serverOpts := []server.ServerOption{server.WithLLMClient(llmClient)}

	ocrAddr := GetString(cmd, FlagOcrAPIAddr)
	zefixDsn := GetString(cmd, FlagZefixDsn)
	if ocrAddr != "" {
		opts := commandIndexerOptions(cmd, llmClient)
		opts = append(opts, indexer.WithOcrApiCAPath(GetString(cmd, FlagOcrCaPath)))

		idx, err := initializeIndexer(ctx, 3, time.Second, func() (*indexer.Indexer, error) {
			attemptCtx, cancelAttempt := context.WithTimeout(ctx, 30*time.Second)
			defer cancelAttempt()
			return indexer.NewWithContext(attemptCtx,
				GetString(cmd, FlagOsAddr),
				ocrAddr,
				zefixDsn,
				opts...,
			)
		})
		if err != nil {
			return fmt.Errorf("initialize configured ingestion: %w", err)
		} else {
			serverOpts = append(serverOpts, server.WithIndexer(idx))
			if zefixDsn != "" {
				ui.PrintSuccess("Indexer initialized — upload endpoint enabled (Zefix enabled)")
			} else {
				ui.PrintSuccess("Indexer initialized — upload endpoint enabled (Zefix disabled)")
			}
		}
	} else {
		ui.PrintWarning("OCR API address not set — upload endpoint will be unavailable")
	}

	listenAddr := GetString(cmd, FlagListenAddr)

	serverOpts = append(serverOpts,
		server.WithAPIToken(GetString(cmd, FlagAPIToken)),
		server.WithTLS(GetString(cmd, FlagTLSCertPath), GetString(cmd, FlagTLSKeyPath)),
	)

	serverInitCtx, cancelServerInit := context.WithTimeout(ctx, 30*time.Second)
	s, err := server.NewWithContext(serverInitCtx,
		GetString(cmd, FlagOsAddr),
		GetString(cmd, FlagOsUsername),
		GetString(cmd, FlagOsPassword),
		GetBool(cmd, FlagOsSkipTLS),
		GetString(cmd, FlagOsIndex),
		storage,
		serverOpts...,
	)
	cancelServerInit()
	if err != nil {
		ui.PrintErrorf("Failed to create server: %v", err)
		return err
	}

	// signal.NotifyContext returns a context that is cancelled on the first
	// SIGINT / SIGTERM. Server.Run blocks on that context for graceful
	// shutdown.
	ui.PrintSuccessf("Starting server on %s", listenAddr)
	if err := s.Run(ctx, listenAddr); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// validateServerListener requires explicit authorization for tokenless public
// listeners. Hostnames are conservative: only literal loopback addresses pass.
func validateServerListener(addr, token string, allowPublic bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if strings.TrimSpace(token) == "" && !allowPublic && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("non-loopback listener requires API_TOKEN or --allow-unauthenticated-public")
	}
	return nil
}

func initializeIndexer(ctx context.Context, attempts int, backoff time.Duration, initialize func() (*indexer.Indexer, error)) (*indexer.Indexer, error) {
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		idx, err := initialize()
		if err == nil {
			return idx, nil
		}
		lastErr = err
		if attempt+1 == attempts {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("dependency initialization failed after %d attempts: %w", attempts, lastErr)
}
