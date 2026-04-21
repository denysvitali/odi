package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

func testIndexer(t *testing.T, handler http.HandlerFunc) *Indexer {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{srv.URL},
		},
	})
	if err != nil {
		t.Fatalf("create OpenSearch client: %v", err)
	}

	return &Indexer{
		documentsIndex:   DefaultDocumentsIndex,
		opensearchClient: client,
	}
}

func TestCreateOpensearchIndexUsesExistingAlias(t *testing.T) {
	var createCalled bool
	idx := testIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			createCalled = true
			w.WriteHeader(http.StatusMethodNotAllowed)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	if err := idx.createOpensearchIndex(context.Background()); err != nil {
		t.Fatalf("createOpensearchIndex returned error: %v", err)
	}
	if createCalled {
		t.Fatal("createOpensearchIndex attempted to create an index even though an alias exists")
	}
}

func TestCreateOpensearchIndexCreatesMissingIndex(t *testing.T) {
	var createCalled bool
	idx := testIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			createCalled = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"acknowledged": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	if err := idx.createOpensearchIndex(context.Background()); err != nil {
		t.Fatalf("createOpensearchIndex returned error: %v", err)
	}
	if !createCalled {
		t.Fatal("createOpensearchIndex did not create a missing index")
	}
}
