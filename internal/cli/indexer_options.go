package cli

import (
	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/llm"
	"github.com/spf13/cobra"
)

// commandIndexerOptions shares OpenSearch and enrichment configuration across
// commands. Callers retain their required-flag checks, OCR transport options and
// initialization lifetimes; the LLM client is supplied to avoid rebuilding it.
func commandIndexerOptions(cmd *cobra.Command, llmClient *llm.Client) []indexer.Option {
	opts := []indexer.Option{
		indexer.WithOpenSearchUsername(GetString(cmd, FlagOsUsername)),
		indexer.WithOpenSearchPassword(GetString(cmd, FlagOsPassword)),
		indexer.WithOpenSearchIndex(GetString(cmd, FlagOsIndex)),
	}
	if GetBool(cmd, FlagOsSkipTLS) {
		opts = append(opts, indexer.WithOpenSearchSkipTLS())
	}
	if llmClient != nil {
		opts = append(opts, indexer.WithLLMClient(llmClient))
	}
	return opts
}
