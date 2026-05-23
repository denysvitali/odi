package cli

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denysvitali/odi/internal/server"
	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/indexer"
)

const (
	FlagListenAddr  = "listen-addr"
	FlagAPIToken    = "api-token"
	FlagTLSCertPath = "tls-cert-path"
	FlagTLSKeyPath  = "tls-key-path"
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
	serveCmd.Flags().StringP(FlagListenAddr, "L", "0.0.0.0:8085", "Address to listen on")
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

	AddOpenSearchFlags(serveCmd)
	AddStorageFlags(serveCmd)
	AddOCRFlags(serveCmd)
	AddZefixFlags(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	log := logrus.StandardLogger()

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
		return nil
	}

	var serverOpts []server.ServerOption

	ocrAddr := GetString(cmd, FlagOcrAPIAddr)
	zefixDsn := GetString(cmd, FlagZefixDsn)
	if ocrAddr != "" {
		opts := []indexer.Option{
			indexer.WithOpenSearchUsername(GetString(cmd, FlagOsUsername)),
			indexer.WithOpenSearchPassword(GetString(cmd, FlagOsPassword)),
			indexer.WithOpenSearchIndex(GetString(cmd, FlagOsIndex)),
			indexer.WithOcrApiCAPath(GetString(cmd, FlagOcrCaPath)),
		}
		if GetBool(cmd, FlagOsSkipTLS) {
			opts = append(opts, indexer.WithOpenSearchSkipTLS())
		}

		idx, err := indexer.New(
			GetString(cmd, FlagOsAddr),
			ocrAddr,
			zefixDsn,
			opts...,
		)
		if err != nil {
			ui.PrintWarningf("Failed to initialize indexer (upload will be unavailable): %v", err)
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

	s, err := server.New(
		GetString(cmd, FlagOsAddr),
		GetString(cmd, FlagOsUsername),
		GetString(cmd, FlagOsPassword),
		GetBool(cmd, FlagOsSkipTLS),
		GetString(cmd, FlagOsIndex),
		storage,
		serverOpts...,
	)
	if err != nil {
		ui.PrintErrorf("Failed to create server: %v", err)
		return err
	}

	// signal.NotifyContext returns a context that is cancelled on the first
	// SIGINT / SIGTERM. Server.Run blocks on that context for graceful
	// shutdown.
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ui.PrintSuccessf("Starting server on %s", listenAddr)
	err = s.Run(ctx, listenAddr)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
	return nil
}
