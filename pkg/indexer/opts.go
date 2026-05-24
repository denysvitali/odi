package indexer

import "github.com/denysvitali/odi/pkg/llm"

func WithOpenSearchUsername(username string) Option {
	return func(i *Indexer) {
		i.opensearchUsername = username
	}
}

func WithOpenSearchPassword(password string) Option {
	return func(i *Indexer) {
		i.opensearchPassword = password
	}
}

func WithOpenSearchSkipTLS() Option {
	return func(i *Indexer) {
		i.opensearchInsecureSkipVerify = true
	}
}

func WithOcrApiCAPath(path string) Option {
	return func(i *Indexer) {
		i.ocrAPICaPath = path
	}
}

func WithOpenSearchIndex(index string) Option {
	return func(i *Indexer) {
		if index != "" {
			i.documentsIndex = index
		}
	}
}

// WithLLMClient enables LLM-based metadata extraction (title, company) during indexing.
// The client is expected to be pre-configured (e.g. pointing at a local Ollama instance).
// If nil or if the client is unhealthy, extraction is silently skipped.
func WithLLMClient(client *llm.Client) Option {
	return func(i *Indexer) {
		i.llmClient = client
	}
}
