package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
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
	"github.com/denysvitali/odi/pkg/thumbnailer"
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
		transport = http.DefaultTransport
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
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: false,
	}))

	s.e.GET("/healthz", s.handleHealthz)
	s.e.GET("/readyz", s.handleReadyz)
	s.e.GET("/metrics", metricsHandler())

	g := s.e.Group("/api/v1")
	if s.apiToken == "" {
		log.Warn("API_TOKEN not set — server running without authentication")
	} else {
		g.Use(authMiddleware(s.apiToken))
	}
	g.POST("/search", s.handleSearch)
	g.GET("/documents/:id", s.handleGetDocument)
	g.GET("/documents", s.handleGetDocuments)
	g.GET("/files/:scanID/:sequenceId", s.handleGetFile)
	g.GET("/thumbnails/:id", s.handleGetThumbnail)
	g.POST("/thumbnails/process", s.handleProcessMissingThumbnails)
	g.POST("/upload", s.handleUpload)
}

type SearchRequest struct {
	SearchTerm string `json:"searchTerm"`
	Size       int    `json:"size,omitempty"`
	ScrollId   string `json:"scrollId,omitempty"`
}

func (s *Server) handleSearch(c *gin.Context) {
	var searchRequest SearchRequest
	err := c.BindJSON(&searchRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	if searchRequest.ScrollId != "" {
		// Continue pagination using scroll (extend TTL to keep context alive)
		scrollResp, err := s.osClient.Scroll.Get(c.Request.Context(), opensearchapi.ScrollGetReq{
			ScrollID: searchRequest.ScrollId,
			Params: opensearchapi.ScrollGetParams{
				Scroll: 10 * time.Minute,
			},
		})
		if err != nil {
			log.Errorf("unable to perform scroll (scrollId=%q): %v", searchRequest.ScrollId, err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		if scrollResp.Inspect().Response.StatusCode >= 400 {
			log.Errorf("scroll returned error (scrollId=%q): %s", searchRequest.ScrollId, scrollResp.Inspect().Response.Status())
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "application/json")
		_, err = io.Copy(c.Writer, scrollResp.Inspect().Response.Body)
		if err != nil {
			logStreamError(c, err, fmt.Sprintf("unable to stream scroll response (scrollId=%q)", searchRequest.ScrollId))
		}
		scrollResp.Inspect().Response.Body.Close()
		return
	}

	// Initial search with scroll enabled
	size := searchRequest.Size
	if size <= 0 {
		size = 50
	}
	if size > maxSearchSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("size exceeds maximum of %d", maxSearchSize)})
		return
	}

	searchContent := map[string]any{
		"size": size,
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  searchRequest.SearchTerm,
				"fields": []string{"text"},
			},
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"text": map[string]any{},
			},
		},
	}

	jsonBody, marshalErr := json.Marshal(searchContent)
	if marshalErr != nil {
		log.Errorf("unable to marshal search body for term=%q size=%d: %v", searchRequest.SearchTerm, size, marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	searchResp, err := s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
		Params: opensearchapi.SearchParams{
			Scroll: 10 * time.Minute,
		},
	})
	if err != nil {
		log.Errorf("unable to perform search (term=%q size=%d): %v", searchRequest.SearchTerm, size, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("search returned error (term=%q): %s", searchRequest.SearchTerm, searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/json")
	_, err = io.Copy(c.Writer, searchResp.Inspect().Response.Body)
	if err != nil {
		logStreamError(c, err, fmt.Sprintf("unable to stream search response (term=%q)", searchRequest.SearchTerm))
	}
	searchResp.Inspect().Response.Body.Close()
}

func (s *Server) returnDocument(c *gin.Context, scanID string, sequenceIdStr string) {
	sequenceId, err := strconv.ParseInt(sequenceIdStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	page, err := s.storage.Retrieve(c.Request.Context(), scanID, int(sequenceId))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, model.ErrNotFound) {
			log.Debugf("page not found: scan=%s seq=%d", scanID, sequenceId)
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("page not found: scan=%s seq=%d", scanID, sequenceId),
			})
			return
		}
		log.Errorf("unable to retrieve page scan=%s seq=%d: %v", scanID, sequenceId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.Status(http.StatusOK)
	_, err = io.Copy(c.Writer, page.Reader)
	if err != nil {
		logStreamError(c, err, fmt.Sprintf("unable to stream page scan=%s seq=%d", scanID, sequenceId))
		return
	}
}

func (s *Server) handleGetFile(c *gin.Context) {
	scanID := c.Param("scanID")
	sequenceIdStr := c.Param("sequenceId")

	if scanID == "" || sequenceIdStr == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	s.returnDocument(c, scanID, sequenceIdStr)
}

func (s *Server) handleGetThumbnail(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	matches := docIdRegexp.FindStringSubmatch(id)
	if len(matches) != 3 {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	scanID := matches[1]
	sequenceIdStr := matches[2]
	sequenceId, err := strconv.ParseInt(sequenceIdStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	// Try thumbnail storage first
	if ts, ok := s.storage.(model.ThumbnailStorage); ok {
		ctx := c.Request.Context()

		exists, err := ts.ThumbnailExists(ctx, scanID, int(sequenceId))
		if err != nil {
			log.Warnf("error checking thumbnail existence: %v", err)
		} else if exists {
			thumb, err := ts.RetrieveThumbnail(ctx, scanID, int(sequenceId))
			if err != nil {
				log.Warnf("error retrieving thumbnail: %v", err)
			} else {
				c.Header("Content-Type", "image/jpeg")
				c.Status(http.StatusOK)
				_, err = io.Copy(c.Writer, thumb.Reader)
				if err != nil {
					logStreamError(c, err, "unable to stream thumbnail")
				}
				return
			}
		}

		// Thumbnail doesn't exist, generate it
		log.Debugf("generating thumbnail for %s_%d", scanID, sequenceId)
		page, err := s.storage.Retrieve(ctx, scanID, int(sequenceId))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, model.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
				return
			}
			log.Errorf("error retrieving original page: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}

		// Generate thumbnail
		_, err = page.Reader.Seek(0, io.SeekStart)
		if err != nil {
			log.Errorf("error seeking page reader: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}

		thumbReader, err := thumbnailer.Generate(page.Reader)
		if err != nil {
			log.Errorf("error generating thumbnail: %v", err)
			// Fall back to original
			s.returnDocument(c, scanID, sequenceIdStr)
			return
		}

		// Store thumbnail for future requests
		err = ts.StoreThumbnail(ctx, scanID, int(sequenceId), thumbReader)
		if err != nil {
			log.Warnf("error storing thumbnail: %v", err)
			// Continue anyway - we still have the thumbnail to return
		}

		// Return thumbnail
		c.Header("Content-Type", "image/jpeg")
		c.Status(http.StatusOK)
		if rs, ok := thumbReader.(io.ReadSeeker); ok {
			_, err = rs.Seek(0, io.SeekStart)
			if err != nil {
				log.Errorf("error seeking generated thumbnail: %v", err)
				return
			}
		}
		_, err = io.Copy(c.Writer, thumbReader)
		if err != nil {
			logStreamError(c, err, "unable to stream generated thumbnail")
		}
		return
	}

	// Thumbnail storage not available, fall back to original
	log.Warn("storage does not implement ThumbnailStorage, returning original image")
	s.returnDocument(c, scanID, sequenceIdStr)
}

var badRequest = gin.H{
	"error": "bad request",
}

var internalServerError = gin.H{
	"error": "internal server error",
}

type Document[T any] struct {
	Index       string `json:"_index"`
	Id          string `json:"_id"`
	Version     int    `json:"_version"`
	SeqNo       int    `json:"_seq_no"`
	PrimaryTerm int    `json:"_primary_term"`
	Found       bool   `json:"found"`
	Source      T      `json:"_source"`
}

var docIdRegexp = regexp.MustCompile("^([0-9a-f-]+)_([0-9]+)$")

func (s *Server) handleGetDocument(c *gin.Context) {
	docId := c.Param("id")
	if docId == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	if !docIdRegexp.MatchString(docId) {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	docResp, err := s.osClient.Document.Get(c.Request.Context(), opensearchapi.DocumentGetReq{
		Index:      s.osIndex,
		DocumentID: docId,
	})
	if err != nil {
		log.Errorf("unable to fetch document %s from OpenSearch: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if docResp.Inspect().Response.StatusCode >= 400 {
		log.Warnf("unable to get document %s: %s", docId, docResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.JSON(http.StatusOK, docResp.Source)
}

func (s *Server) handleGetDocuments(c *gin.Context) {
	scrollId := c.Query("scroll_id")
	var searchResp *opensearchapi.SearchResp
	var err error

	if scrollId != "" {
		// Continue scroll (extend TTL so paging stays alive across many loads)
		scrollResp, err := s.osClient.Scroll.Get(c.Request.Context(), opensearchapi.ScrollGetReq{
			ScrollID: scrollId,
			Params: opensearchapi.ScrollGetParams{
				Scroll: 10 * time.Minute,
			},
		})
		if err != nil {
			log.Errorf("unable to get documents (scroll): %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		if scrollResp.Inspect().Response.StatusCode >= 400 {
			log.Warnf("unable to get documents (scroll): %s", scrollResp.Inspect().Response.Status())
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		c.Status(http.StatusOK)
		c.Header("Content-Type", "application/json")
		_, err = io.Copy(c.Writer, scrollResp.Inspect().Response.Body)
		if err != nil {
			logStreamError(c, err, "unable to stream documents response (scroll)")
		}
		scrollResp.Inspect().Response.Body.Close()
		return
	}

	// Initial search with scroll
	size := 50
	if sizeStr := c.Query("size"); sizeStr != "" {
		if parsed, parseErr := strconv.Atoi(sizeStr); parseErr == nil && parsed > 0 {
			size = parsed
		}
	}
	if size > maxSearchSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("size exceeds maximum of %d", maxSearchSize)})
		return
	}

	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		t, err := time.Parse("2006-01-02", dateFromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid date_from: %v", err)})
			return
		}
		dateFrom = &t
	}
	if dateToStr := c.Query("date_to"); dateToStr != "" {
		t, err := time.Parse("2006-01-02", dateToStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid date_to: %v", err)})
			return
		}
		endOfDay := t.Add(24*time.Hour - time.Second)
		dateTo = &endOfDay
	}

	hasDateFilter := dateFrom != nil || dateTo != nil

	searchBody := map[string]any{
		"size": size,
		"sort": []map[string]any{{"indexedAt": "desc"}},
	}

	if hasDateFilter {
		var filters []map[string]any
		if dateFrom != nil {
			filters = append(filters, map[string]any{
				"range": map[string]any{"date": map[string]any{"gte": dateFrom.Format(time.RFC3339)}},
			})
		}
		if dateTo != nil {
			filters = append(filters, map[string]any{
				"range": map[string]any{"date": map[string]any{"lte": dateTo.Format(time.RFC3339)}},
			})
		}
		searchBody["query"] = map[string]any{
			"bool": map[string]any{"filter": filters},
		}
	}
	jsonBody, marshalErr := json.Marshal(searchBody)
	if marshalErr != nil {
		log.Errorf("unable to marshal search body: %v", marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	searchResp, err = s.osClient.Search(c.Request.Context(), &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
		Params: opensearchapi.SearchParams{
			Scroll: 10 * time.Minute,
		},
	})
	if err != nil {
		log.Errorf("unable to get documents: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Warnf("unable to get documents: %s", searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/json")
	_, err = io.Copy(c.Writer, searchResp.Inspect().Response.Body)
	if err != nil {
		logStreamError(c, err, "unable to stream documents response")
	}
	searchResp.Inspect().Response.Body.Close()
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
