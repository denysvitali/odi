package cli

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denysvitali/odi/pkg/logutils"
)

var rootCmd = &cobra.Command{
	Use:   "odi",
	Short: "ODI - Open Document Indexer",
	Long: `ODI (Open Document Indexer) is a document digitization and indexing system.

It provides tools for:
  - Ingesting documents from network scanners
  - OCR processing and text extraction
  - Indexing documents in OpenSearch
  - Searching and retrieving documents via REST API
  - Secure document encryption/decryption`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup logging
		logLevel := GetString(cmd, FlagLogLevel)
		logutils.SetupLogger(logLevel)

		// Try to fill keychain values for any struct that might need it
		return nil
	},
	SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().String(FlagLogLevel, "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String(FlagConfig, "", "Config file path")

	_ = viper.BindPFlag(FlagLogLevel, rootCmd.PersistentFlags().Lookup(FlagLogLevel))
	bindEnv(FlagLogLevel, "LOG_LEVEL")

	// Add all subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(pdfCmd)
	rootCmd.AddCommand(reindexCmd)
	rootCmd.AddCommand(ocrCmd)
	rootCmd.AddCommand(decryptCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(zefixImportCmd)
	rootCmd.AddCommand(zefixFindCmd)
}

func initConfig() {
	// Load .env file if it exists (doesn't error if missing)
	_ = godotenv.Load()

	// Set up viper
	viper.SetEnvPrefix("ODI")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	// Also check for legacy env vars without ODI_ prefix
	for _, envVar := range []string{
		"OPENSEARCH_ADDR", "OPENSEARCH_USERNAME", "OPENSEARCH_PASSWORD",
		"OPENSEARCH_SKIP_TLS", "OPENSEARCH_INDEX",
		"STORAGE_TYPE", "B2_ACCOUNT", "B2_KEY", "B2_BUCKET_NAME", "B2_PASSPHRASE",
		"FS_PATH", "OCR_API_ADDR", "OCR_API_CA_PATH", "ZEFIX_DSN",
		"LOG_LEVEL", "SCANNER_NAME", "SOURCE", "PASSPHRASE",
		"BACKEND", "BACKEND_URL", "BACKEND_TOKEN",
	} {
		if val := os.Getenv(envVar); val != "" {
			viper.SetDefault(strings.ToLower(strings.ReplaceAll(envVar, "_", "-")), val)
		}
	}

	// Load config file. We can't bind --config via BindPFlag here because the
	// persistent flag is parsed after OnInitialize runs, so we look up the
	// raw flag value from os.Args / env ODI_CONFIG.
	explicit := os.Getenv("ODI_CONFIG")
	for i, a := range os.Args {
		if a == "--config" && i+1 < len(os.Args) {
			explicit = os.Args[i+1]
			break
		}
		if strings.HasPrefix(a, "--config=") {
			explicit = strings.TrimPrefix(a, "--config=")
			break
		}
	}
	if err := loadConfigFile(explicit); err != nil {
		// Fatal only for an explicitly requested file that failed to load.
		if explicit != "" {
			panic(err)
		}
	}
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
