package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	zefixpkg "github.com/denysvitali/odi/zefix-tools/pkg/zefix"
)

const (
	FlagInputFile = "input-file"
)

var zefixImportCmd = &cobra.Command{
	Use:   "zefix-import",
	Short: "Import Zefix company data into PostgreSQL",
	Long: `Import Swiss commercial register (Zefix) data from a JSON file into PostgreSQL.

This command reads a SPARQL-exported Zefix JSON file and imports the company
records into a PostgreSQL database for cross-referencing during document indexing.`,
	Args: cobra.NoArgs,
	RunE: runZefixImport,
}

func init() {
	zefixImportCmd.Flags().StringP(FlagInputFile, "i", "", "Input JSON file path (env: INPUT_FILE)")
	bindEnv(FlagInputFile, "INPUT_FILE")
	AddZefixFlags(zefixImportCmd)
}

func runZefixImport(cmd *cobra.Command, args []string) error {
	inputFile := GetString(cmd, FlagInputFile)
	dsn := GetString(cmd, FlagZefixDsn)

	if dsn == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: ZEFIX_DSN)", FlagZefixDsn)
	}
	if inputFile == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: INPUT_FILE)", FlagInputFile)
	}

	c, err := zefixpkg.New(dsn)
	if err != nil {
		ui.PrintErrorf("Failed to create Zefix client: %v", err)
		return err
	}

	err = c.Import(inputFile)
	if err != nil {
		ui.PrintErrorf("Failed to import: %v", err)
		return err
	}

	ui.PrintSuccess("Import complete")
	return nil
}
