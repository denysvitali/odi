package cli

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denysvitali/odi/internal/server"
	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/indexer"
)

const (
	FlagListenAddr = "listen-addr"
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

	ui.PrintSuccessf("Starting server on %s", listenAddr)
	err = s.Run(listenAddr)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
	return nil
}
