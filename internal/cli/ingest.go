package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	"github.com/denysvitali/odi/pkg/ingestor"
)

const (
	FlagScannerName  = "scanner-name"
	FlagSource       = "source"
	FlagBackend      = "backend"
	FlagBackendURL   = "backend-url"
	FlagBackendToken = "backend-token"

	BackendLocal  = "local"
	BackendRemote = "remote"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest documents from a network scanner",
	Long: `Ingest documents from a network scanner via AirScan protocol.

The backend target decides where the scanned pages are processed:
  - local:  store + OCR + index in-process (requires OpenSearch, OCR, Zefix, storage).
  - remote: POST each scan to an odi server's /api/v1/upload endpoint.
            Only --backend-url (and optionally --backend-token) are required.

Configuration can be supplied via flags, ODI_* environment variables, or a
YAML config file loaded from $XDG_CONFIG_HOME/odi/config.yaml (default
~/.config/odi/config.yaml) or /etc/odi/config.yaml. Use --config to point at
a specific file.`,
	RunE: runIngest,
}

func init() {
	ingestCmd.Flags().String(FlagScannerName, "", "Name of the scanner to use (env: SCANNER_NAME)")
	ingestCmd.Flags().String(FlagSource, "Feeder", "Document source: Feeder or Platen (env: SOURCE)")
	ingestCmd.Flags().String(FlagBackend, BackendLocal, "Backend target: local or remote (env: BACKEND)")
	ingestCmd.Flags().String(FlagBackendURL, "", "Remote backend base URL, e.g. https://odi.example.com (env: BACKEND_URL)")
	ingestCmd.Flags().String(FlagBackendToken, "", "Optional bearer token for the remote backend (env: BACKEND_TOKEN)")

	bindEnv(FlagScannerName, "SCANNER_NAME")
	bindEnv(FlagSource, "SOURCE")
	bindEnv(FlagBackend, "BACKEND")
	bindEnv(FlagBackendURL, "BACKEND_URL")
	bindEnv(FlagBackendToken, "BACKEND_TOKEN")

	AddOpenSearchFlags(ingestCmd)
	AddStorageFlags(ingestCmd)
	AddOCRFlags(ingestCmd)
	AddZefixFlags(ingestCmd)
}

func runIngest(cmd *cobra.Command, args []string) error {
	if GetString(cmd, FlagScannerName) == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: SCANNER_NAME)", FlagScannerName)
	}

	backend, err := buildIngestBackend(cmd)
	if err != nil {
		ui.PrintErrorf("Failed to initialize backend: %v", err)
		return err
	}

	scannerName := GetString(cmd, FlagScannerName)
	source := GetString(cmd, FlagSource)

	ui.PrintInfof("Connecting to scanner: %s", scannerName)

	i := ingestor.NewWithBackend(backend)
	defer i.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := i.Ping(ctx); err != nil {
		ui.PrintErrorf("Backend not reachable: %v", err)
		return err
	}

	ui.PrintInfo("Starting document ingestion...")
	if err := i.Ingest(ctx, scannerName, source); err != nil {
		ui.PrintErrorf("Ingestion failed: %v", err)
		return err
	}
	ui.PrintSuccess("Ingestion completed successfully")
	return nil
}

func buildIngestBackend(cmd *cobra.Command) (ingestor.Backend, error) {
	kind := resolveBackendKind(cmd)
	switch kind {
	case BackendLocal:
		return buildLocalBackend(cmd)
	case BackendRemote:
		return buildRemoteBackend(cmd)
	default:
		return nil, fmt.Errorf("unknown backend %q (expected %q or %q)", kind, BackendLocal, BackendRemote)
	}
}

func buildLocalBackend(cmd *cobra.Command) (ingestor.Backend, error) {
	for _, required := range []struct{ flag, env string }{
		{FlagOsAddr, "OPENSEARCH_ADDR"},
		{FlagOcrAPIAddr, "OCR_API_ADDR"},
		{FlagZefixDsn, "ZEFIX_DSN"},
		{FlagStorageType, "STORAGE_TYPE"},
	} {
		if GetString(cmd, required.flag) == "" {
			return nil, fmt.Errorf("local backend: required flag or env var not set: %s (env: %s)", required.flag, required.env)
		}
	}
	storage, err := GetStorage(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}
	if storage == nil {
		return nil, fmt.Errorf("invalid storage type: must be 'b2' or 'fs'")
	}
	return ingestor.NewLocalBackend(ingestor.Config{
		OcrAPIAddr:         GetString(cmd, FlagOcrAPIAddr),
		OpenSearchAddr:     GetString(cmd, FlagOsAddr),
		OpenSearchPassword: GetString(cmd, FlagOsPassword),
		OpenSearchSkipTLS:  GetBool(cmd, FlagOsSkipTLS),
		OpenSearchUsername: GetString(cmd, FlagOsUsername),
		Storage:            storage,
		ZefixDsn:           GetString(cmd, FlagZefixDsn),
	})
}

func buildRemoteBackend(cmd *cobra.Command) (ingestor.Backend, error) {
	url := GetString(cmd, FlagBackendURL)
	if url == "" {
		return nil, fmt.Errorf("remote backend: required flag or env var not set: %s (env: BACKEND_URL)", FlagBackendURL)
	}
	return ingestor.NewRemoteBackend(ingestor.RemoteBackendConfig{
		BaseURL: url,
		Token:   GetString(cmd, FlagBackendToken),
	})
}
