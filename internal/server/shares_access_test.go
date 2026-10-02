package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/denysvitali/odi/pkg/llm"
	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func shareAccessServer(t *testing.T, rec shareRecord, update http.HandlerFunc) (*Server, string) {
	t.Helper()
	token, err := signShare([]byte("synthetic-api-secret"), SharePayload{ScanID: rec.ScanID, SequenceID: rec.SequenceID, ExpiresAt: rec.ExpiresAt, MaxViews: rec.MaxViews})
	require.NoError(t, err)
	rec.Token = token
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"_index": sharesIndex, "_id": token, "found": true, "_source": rec})
			return
		}
		update(w, r)
	}))
	t.Cleanup(upstream.Close)
	osClient, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{upstream.URL}}})
	require.NoError(t, err)
	store := newMockRWStorage()
	store.addPage(rec.ScanID, rec.SequenceID, []byte("synthetic-document"))
	s := &Server{e: gin.New(), osClient: osClient, storage: store, apiToken: "synthetic-api-secret"}
	s.e.GET("/share/:token", s.handleServeShare)
	return s, token
}

func TestShareAccessRequiresCommittedReservation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		result string
		want   int
	}{
		{"committed", http.StatusOK, "updated", http.StatusOK},
		{"policy changed", http.StatusOK, "noop", http.StatusNotFound},
		{"accounting failure", http.StatusInternalServerError, "", http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := shareRecord{ScanID: "synthetic-scan", SequenceID: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(), MaxViews: 1}
			s, token := shareAccessServer(t, rec, func(w http.ResponseWriter, r *http.Request) {
				body := decodeSearchBody(t, r)
				script := body["script"].(map[string]any)
				source := script["source"].(string)
				for _, field := range []string{"revoked", "expiresAt", "maxViews", "viewCount", "ctx.op = 'noop'"} {
					require.Contains(t, source, field)
				}
				require.Greater(t, script["params"].(map[string]any)["now"].(float64), float64(0))
				w.WriteHeader(tt.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"_index": sharesIndex, "result": tt.result})
			})
			w := httptest.NewRecorder()
			s.e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/share/"+token, nil))
			require.Equal(t, tt.want, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
			if tt.want == http.StatusOK {
				require.Equal(t, "synthetic-document", w.Body.String())
			} else {
				require.NotContains(t, w.Body.String(), "synthetic-document")
			}
		})
	}
}

func TestShareConcurrentAccessOnlyStreamsCommittedViews(t *testing.T) {
	rec := shareRecord{ScanID: "synthetic-scan", SequenceID: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(), MaxViews: 1}
	var mu sync.Mutex
	consumed := 0
	s, token := shareAccessServer(t, rec, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		result := "noop"
		if consumed < rec.MaxViews {
			consumed++
			result = "updated"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"_index": sharesIndex, "result": result})
	})
	const requests = 8
	results := make(chan int, requests)
	var wg sync.WaitGroup
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			s.e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/share/"+token, nil))
			results <- w.Code
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for status := range results {
		if status == http.StatusOK {
			successes++
		} else {
			require.Equal(t, http.StatusNotFound, status)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, consumed)
}

func TestSharePassphraseUsesHeaderOnly(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("synthetic-passphrase"), bcrypt.MinCost)
	require.NoError(t, err)
	for _, tt := range []struct {
		name   string
		query  string
		header string
		want   int
	}{
		{"missing", "", "", http.StatusNotFound},
		{"incorrect header", "", "wrong", http.StatusNotFound},
		{"URL query ignored", "?p=synthetic-passphrase", "", http.StatusNotFound},
		{"header accepted", "", "synthetic-passphrase", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := shareRecord{ScanID: "synthetic-scan", SequenceID: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(), PassphraseHash: string(hash)}
			s, token := shareAccessServer(t, rec, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.StatusOK, tt.want)
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "updated"})
			})
			req := httptest.NewRequest(http.MethodGet, "/share/"+token+tt.query, nil)
			req.Header.Set("X-Share-Passphrase", tt.header)
			w := httptest.NewRecorder()
			s.e.ServeHTTP(w, req)
			require.Equal(t, tt.want, w.Code)
		})
	}
}

func TestShareRejectsExpiredAndRevokedBeforeAccounting(t *testing.T) {
	for _, tt := range []struct {
		name    string
		revoked bool
		expiry  int64
	}{
		{"expired", false, time.Now().Add(-time.Hour).Unix()},
		{"revoked", true, time.Now().Add(time.Hour).Unix()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := shareRecord{ScanID: "synthetic-scan", SequenceID: 1, ExpiresAt: tt.expiry, Revoked: tt.revoked}
			s, token := shareAccessServer(t, rec, func(w http.ResponseWriter, r *http.Request) { t.Error("rejected share reached accounting") })
			w := httptest.NewRecorder()
			s.e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/share/"+token, nil))
			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestShareReservationCancellation(t *testing.T) {
	rec := shareRecord{ScanID: "synthetic-scan", SequenceID: 1, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	s, token := shareAccessServer(t, rec, func(w http.ResponseWriter, r *http.Request) { t.Error("canceled reservation reached upstream") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reserved, err := s.reserveShareView(ctx, token, time.Now().Unix())
	require.Error(t, err)
	require.False(t, reserved)
	require.NotContains(t, err.Error(), token)
}

func TestChatUsesInjectedClient(t *testing.T) {
	t.Setenv("LLM_API_ADDR", "http://127.0.0.1:1")
	s := &Server{e: gin.New()}
	s.e.POST("/chat", s.handleChat)
	w := httptest.NewRecorder()
	s.e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"question":"synthetic question"}`)))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestChatInjectedModelAndEndpoint(t *testing.T) {
	t.Setenv("LLM_API_ADDR", "http://127.0.0.1:1")
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeSearchBody(t, r)
		require.Equal(t, "synthetic-model", body["model"])
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role": "assistant", "content": "synthetic answer"}})
	}))
	defer llmServer.Close()
	c, err := llm.New(llmServer.URL, llm.WithModel("synthetic-model"))
	require.NoError(t, err)
	ms := searchTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(osSearchResponse([]map[string]any{{"_id": "synthetic-scan_1", "_source": map[string]string{"text": "synthetic text", "title": "synthetic title"}}}, 1))
	})
	ms.llmClient = c
	ms.router.POST("/chat", ms.handleChat)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"question":"synthetic question"}`)))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "synthetic answer")
}
