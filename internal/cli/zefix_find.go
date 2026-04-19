package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denysvitali/odi/internal/ui"
	zefixpkg "github.com/denysvitali/odi/zefix-tools/pkg/zefix"
)

const (
	FlagCompanyName = "company-name"
)

var zefixFindCmd = &cobra.Command{
	Use:   "zefix-find [company-name]",
	Short: "Find a company in the Zefix database",
	Long: `Search for a Swiss company by name in the Zefix PostgreSQL database.

The database must have been previously populated using the zefix-import command.`,
	Args: cobra.ExactArgs(1),
	RunE: runZefixFind,
}

func init() {
	AddZefixFlags(zefixFindCmd)
}

func runZefixFind(cmd *cobra.Command, args []string) error {
	dsn := GetString(cmd, FlagZefixDsn)
	if dsn == "" {
		return fmt.Errorf("required flag or env var not set: %s (env: ZEFIX_DSN)", FlagZefixDsn)
	}

	c, err := zefixpkg.New(dsn)
	if err != nil {
		ui.PrintErrorf("Failed to create Zefix client: %v", err)
		return err
	}

	company, err := c.FindCompany(args[0])
	if err != nil {
		ui.PrintErrorf("Failed to find company: %v", err)
		return err
	}

	if company == nil {
		ui.PrintError("Company not found")
		return nil
	}

	ui.PrintSuccessf("Name: %s", company.Name)
	ui.PrintSuccessf("URI:  %s", company.Uri)
	return nil
}
