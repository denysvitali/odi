package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/storage/model"
)

// maxSearchSize is the hard upper bound on the page size accepted by the
// search endpoints. Larger values are rejected with HTTP 400.
const maxSearchSize = 1000

const (
	serverReadTimeout  = 30 * time.Second
	serverWriteTimeout = 5 * time.Minute
)

type Server struct {
	e                    *gin.Engine
	osUrl                *url.URL
	osUsername           string
	osPassword           string
	osIndex              string
	osInsecureSkipVerify bool
	osClient             *opensearchapi.Client
	storage              model.RWStorage
	indexer              *indexer.Indexer
	thumbnailProcessMu   sync.Mutex
	reindexProcessMu     sync.Mutex
	reindexStatusMu      sync.Mutex
	reindexStatus        reindexStatus
	workerCtx            context.Context

	// apiToken, if non-empty, is required as a Bearer token on /api/v1/*.
	apiToken string

	// tlsCertPath / tlsKeyPath, if both set, switch the server to HTTPS.
	tlsCertPath string
	tlsKeyPath  string
}

var log = logrus.StandardLogger().WithField("package", "server")

// logStreamError records a mid-stream io.Copy failure. By the time we hit
// this path the HTTP headers (and likely some body bytes) have already been
// flushed to the client, so we cannot rewrite the status — we record an
// error log enriched with the request context so operators can correlate
// the partial response with the failure.
func logStreamError(c *gin.Context, err error, msg string) {
	if err == nil {
		return
	}
	log.WithFields(logrus.Fields{
		"request_id": RequestIDFromContext(c.Request.Context()),
		"path":       c.Request.URL.Path,
		"route":      c.FullPath(),
		"status":     c.Writer.Status(),
		"method":     c.Request.Method,
	}).WithError(err).Error(msg)
}

type ServerOption func(*Server)

func WithIndexer(idx *indexer.Indexer) ServerOption {
	return func(s *Server) {
		s.indexer = idx
	}
}

// WithAPIToken configures the bearer token required on /api/v1 routes. An
// empty token leaves authentication disabled (a startup warning is logged
// in that case).
func WithAPIToken(token string) ServerOption {
	return func(s *Server) {
		s.apiToken = token
	}
}

// WithTLS configures the certificate and key paths used to serve HTTPS. If
// either path is empty, the server falls back to plain HTTP.
func WithTLS(certPath, keyPath string) ServerOption {
	return func(s *Server) {
		s.tlsCertPath = certPath
		s.tlsKeyPath = keyPath
	}
}

func New(osAddr string, osUsername string, osPassword string, osInsecureSkipVerify bool, osIndex string, storage model.RWStorage, opts ...ServerOption) (*Server, error) {
	u, err := url.Parse(osAddr)
	if err != nil {
		return nil, fmt.Errorf("parse OpenSearch address %q: %w", osAddr, err)
	}

	s := Server{
		e:                    gin.New(),
		osUrl:                u,
		osUsername:           osUsername,
		osPassword:           osPassword,
		osInsecureSkipVerify: osInsecureSkipVerify,
		osIndex:              osIndex,
		storage:              storage,
	}

	for _, opt := range opts {
		opt(&s)
	}

	var transport http.RoundTripper
	if s.osInsecureSkipVerify {
		log.Warn("OpenSearch TLS verification disabled — do not use in production")
		transport = &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // Intentionally disabled via operator flag for dev/testing only.
			MinVersion:         tls.VersionTLS12,
		}}
	} else {
		transport = http.DefaultTransport.(*http.Transport).Clone()
	}

	c, err := opensearchapi.NewClient(
		opensearchapi.Config{
			Client: opensearch.Config{
				Addresses: []string{s.osUrl.String()},
				Username:  s.osUsername,
				Password:  s.osPassword,
				Transport: transport,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenSearch client for %s: %w", u.String(), err)
	}
	s.osClient = c

	err = s.verifyOpensearch(context.Background(), osIndex)
	if err != nil {
		return nil, fmt.Errorf("verify OpenSearch index %s: %w", osIndex, err)
	}

	s.initRoutes()
	return &s, nil
}

func (s *Server) verifyOpensearch(ctx context.Context, osIndex string) error {
	err := s.pingOs(ctx)
	if err != nil {
		return fmt.Errorf("ping OpenSearch at %s: %w", s.osUrl.String(), err)
	}

	err = s.verifyIndex(ctx, osIndex)
	if err != nil {
		return fmt.Errorf("verify index %s: %w", osIndex, err)
	}
	return nil
}

// Run starts the HTTP server and blocks until the parent context is
// cancelled or a SIGINT/SIGTERM is received. On shutdown it cancels the
// background context used by long-running goroutines (e.g. the thumbnail
// processor) and calls srv.Shutdown with a 30s deadline.
func (s *Server) Run(ctx context.Context, addr string) error {
	// Background context for in-process workers — cancelled on signal so they
	// stop together with the HTTP server.
	bgCtx, cancelBg := context.WithCancel(ctx)
	defer cancelBg()
	s.workerCtx = bgCtx
	s.startThumbnailProcessor(bgCtx)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.e,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
	}

	tlsEnabled := s.tlsCertPath != "" && s.tlsKeyPath != ""

	errCh := make(chan error, 1)
	go func() {
		var err error
		if tlsEnabled {
			log.Infof("listening on https://%s", addr)
			err = srv.ListenAndServeTLS(s.tlsCertPath, s.tlsKeyPath)
		} else {
			log.Infof("listening on http://%s", addr)
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Infof("shutdown signal received: %v", ctx.Err())
	}

	// Stop background goroutines first so they observe the cancellation
	// before we wait for in-flight HTTP requests to drain.
	cancelBg()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("graceful shutdown failed: %v", err)
		return err
	}
	// Drain the goroutine's final result.
	if err := <-errCh; err != nil {
		return err
	}
	return nil
}

func corsOrigins() []string {
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		return strings.Split(v, ",")
	}
	return []string{"http://localhost:5173"}
}

func (s *Server) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) initRoutes() {
	// Request ID must run before any other middleware that wants to log it.
	s.e.Use(requestIDMiddleware())
	s.e.Use(metricsMiddleware())
	s.e.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz", "/readyz", "/metrics"},
		Formatter: func(p gin.LogFormatterParams) string {
			reqID, _ := p.Keys["request_id"].(string)
			logrus.WithFields(logrus.Fields{
				"package":    "server",
				"request_id": reqID,
				"method":     p.Method,
				"path":       p.Path,
				"status":     p.StatusCode,
				"latency":    p.Latency.String(),
				"client_ip":  p.ClientIP,
			}).Info("http request")
			return ""
		},
	}))
	s.e.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins(),
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	s.e.GET("/healthz", s.handleHealthz)
	s.e.GET("/readyz", s.handleReadyz)
	s.e.GET("/metrics", metricsHandler())
	s.e.GET("/share/:token", s.handleServeShare)

	g := s.e.Group("/api/v1")
	if s.apiToken == "" {
		log.Warn("API_TOKEN not set — server running without authentication")
	} else {
		g.Use(authMiddleware(s.apiToken))
	}
	g.POST("/search", s.handleSearch)
	g.POST("/search/facets", s.handleSearchFacets)
	g.GET("/documents/:id", s.handleGetDocument)
	g.GET("/documents", s.handleGetDocuments)
	g.GET("/files/:scanID/:sequenceId", s.handleGetFile)
	g.GET("/thumbnails/:id", s.handleGetThumbnail)
	g.POST("/thumbnails/process", s.handleProcessMissingThumbnails)
	g.POST("/admin/reindex", s.handleStartReindex)
	g.GET("/admin/reindex", s.handleGetReindexStatus)
	g.POST("/upload", s.handleUpload)
	g.POST("/chat", s.handleChat)
	g.POST("/documents/:id/summary", s.handleDocumentSummary)
	g.POST("/shares", s.handleCreateShare)
	g.GET("/shares", s.handleListShares)
	g.DELETE("/shares/:token", s.handleRevokeShare)
}

func (s *Server) pingOs(ctx context.Context) error {
	res, err := s.osClient.Ping(ctx, &opensearchapi.PingReq{})
	if err != nil {
		return fmt.Errorf("ping OpenSearch: %w", err)
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("ping OpenSearch returned %s", res.Status())
	}
	return nil
}

func (s *Server) verifyIndex(ctx context.Context, index string) error {
	resp, err := s.osClient.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{Indices: []string{index}})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("index or alias %s not found", index)
		}
		return fmt.Errorf("check index or alias %s: %w", index, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("index or alias %s not found", index)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unable to verify index or alias %s: %s", index, resp.Status())
	}
	return nil
}
