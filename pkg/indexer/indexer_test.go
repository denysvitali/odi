package indexer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		initCalled:       true,
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

func TestReserveContentDigestCreatesReservation(t *testing.T) {
	idx := testIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents_digests/_create/sha256:abc":
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.URL.Query().Get("refresh") != "true" {
				t.Fatalf("expected refresh=true, got %q", r.URL.RawQuery)
			}
			var body struct {
				DocumentID string `json:"documentID"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if body.DocumentID != "scan_1" {
				t.Fatalf("expected documentID scan_1, got %q", body.DocumentID)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"result":"created"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	reservation, err := idx.ReserveContentDigest(context.Background(), "sha256:abc", "scan_1")
	if err != nil {
		t.Fatalf("ReserveContentDigest returned error: %v", err)
	}
	if !reservation.Reserved {
		t.Fatal("expected digest to be reserved")
	}
	if reservation.ExistingDocumentID != "" {
		t.Fatalf("expected no existing document, got %q", reservation.ExistingDocumentID)
	}
}

func TestReserveContentDigestReturnsExistingDocumentOnConflict(t *testing.T) {
	idx := testIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents_digests/_create/sha256:abc":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"type":"version_conflict_engine_exception"}}`))
		case "/documents_digests/_doc/sha256:abc":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_source":{"documentID":"existing_7","createdAt":"2026-04-21T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	reservation, err := idx.ReserveContentDigest(context.Background(), "sha256:abc", "scan_1")
	if err != nil {
		t.Fatalf("ReserveContentDigest returned error: %v", err)
	}
	if reservation.Reserved {
		t.Fatal("expected digest conflict")
	}
	if reservation.ExistingDocumentID != "existing_7" {
		t.Fatalf("expected existing_7, got %q", reservation.ExistingDocumentID)
	}
}

func TestCreateContentDigestIndexCreatesKeywordMapping(t *testing.T) {
	idx := testIndexer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents_digests":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			buf, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(buf), `"documentID": { "type": "keyword" }`) {
				t.Fatalf("expected documentID keyword mapping, got %s", string(buf))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	if err := idx.createContentDigestIndex(context.Background()); err != nil {
		t.Fatalf("createContentDigestIndex returned error: %v", err)
	}
}
