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
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/odi/pkg/indexer"
	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
	"github.com/denysvitali/odi/pkg/thumbnailer"
)

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
	osClient             *opensearch.Client
	storage              model.RWStorage
	indexer              *indexer.Indexer
}

var log = logrus.StandardLogger().WithField("package", "server")

type ServerOption func(*Server)

func WithIndexer(idx *indexer.Indexer) ServerOption {
	return func(s *Server) {
		s.indexer = idx
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
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	} else {
		transport = http.DefaultTransport
	}

	c, err := opensearch.NewClient(
		opensearch.Config{
			Addresses: []string{s.osUrl.String()},
			Username:  s.osUsername,
			Password:  s.osPassword,
			Transport: transport,
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

func (s *Server) Run(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.e,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
	}
	return srv.ListenAndServe()
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
	s.e.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz"},
	}))
	s.e.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins(),
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: false,
	}))

	s.e.GET("/healthz", s.handleHealthz)

	g := s.e.Group("/api/v1")
	g.POST("/search", s.handleSearch)
	g.GET("/documents/:id", s.handleGetDocument)
	g.GET("/documents", s.handleGetDocuments)
	g.GET("/files/:scanID/:sequenceId", s.handleGetFile)
	g.GET("/thumbnails/:id", s.handleGetThumbnail)
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

	var res *opensearchapi.Response

	if searchRequest.ScrollId != "" {
		// Continue pagination using scroll
		scrollBody := map[string]any{
			"scroll":    "10m",
			"scroll_id": searchRequest.ScrollId,
		}
		jsonBody, marshalErr := json.Marshal(scrollBody)
		if marshalErr != nil {
			log.Errorf("unable to marshal scroll body for scroll_id=%s: %v", searchRequest.ScrollId, marshalErr)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		req := opensearchapi.ScrollRequest{
			Body: bytes.NewReader(jsonBody),
		}
		res, err = req.Do(c.Request.Context(), s.osClient)
	} else {
		// Initial search with scroll enabled
		size := searchRequest.Size
		if size <= 0 {
			size = 50
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

		req := opensearchapi.SearchRequest{
			Index:  []string{s.osIndex},
			Body:   bytes.NewReader(jsonBody),
			Scroll: 10 * time.Minute,
		}
		res, err = req.Do(c.Request.Context(), s.osClient)
	}

	if err != nil {
		log.Errorf("unable to perform search (term=%q scrollId=%q size=%d): %v", searchRequest.SearchTerm, searchRequest.ScrollId, searchRequest.Size, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if res.IsError() {
		log.Errorf("search returned error (term=%q scrollId=%q): %s", searchRequest.SearchTerm, searchRequest.ScrollId, res.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/json")
	_, err = io.Copy(c.Writer, res.Body)
	if err != nil {
		log.Errorf("unable to stream search response (term=%q scrollId=%q): %v", searchRequest.SearchTerm, searchRequest.ScrollId, err)
		return
	}
}

func (s *Server) returnDocument(c *gin.Context, scanID string, sequenceIdStr string) {
	sequenceId, err := strconv.ParseInt(sequenceIdStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	page, err := s.storage.Retrieve(c.Request.Context(), scanID, int(sequenceId))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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
		log.Errorf("unable to stream page scan=%s seq=%d: %v", scanID, sequenceId, err)
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
					log.Errorf("unable to stream thumbnail: %v", err)
				}
				return
			}
		}

		// Thumbnail doesn't exist, generate it
		log.Debugf("generating thumbnail for %s_%d", scanID, sequenceId)
		page, err := s.storage.Retrieve(ctx, scanID, int(sequenceId))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
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
		_, err = thumbReader.(io.ReadSeeker).Seek(0, io.SeekStart)
		if err != nil {
			// thumbReader might not be a ReadSeeker
			c.Header("Content-Type", "image/jpeg")
			c.Status(http.StatusOK)
			_, err = io.Copy(c.Writer, thumbReader)
			if err != nil {
				log.Errorf("unable to stream generated thumbnail: %v", err)
			}
			return
		}

		c.Header("Content-Type", "image/jpeg")
		c.Status(http.StatusOK)
		_, err = io.Copy(c.Writer, thumbReader)
		if err != nil {
			log.Errorf("unable to stream generated thumbnail: %v", err)
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

	req := opensearchapi.GetRequest{Index: s.osIndex, DocumentID: docId}
	res, err := req.Do(c.Request.Context(), s.osClient)
	if err != nil {
		log.Errorf("unable to fetch document %s from OpenSearch: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if res.IsError() {
		log.Warnf("unable to get document %s: %s", docId, res.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	var doc Document[models.Document]
	err = json.NewDecoder(res.Body).Decode(&doc)
	if err != nil {
		log.Errorf("unable to decode document %s: %v", docId, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if !doc.Found {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "not found",
		})
		return
	}

	c.JSON(http.StatusOK, doc.Source)
}

func (s *Server) handleGetDocuments(c *gin.Context) {
	scrollId := c.Query("scrollId")
	var res *opensearchapi.Response
	var err error
	if scrollId != "" {
		// Use Body to pass scroll_id instead of URL query parameter to avoid 405 errors
		// when scroll_id is long. See https://github.com/opensearch-project/opensearch-go/issues/422
		scrollBody := map[string]any{
			"scroll":    "10m",
			"scroll_id": scrollId,
		}
		jsonBody, marshalErr := json.Marshal(scrollBody)
		if marshalErr != nil {
			log.Errorf("unable to marshal scroll body for scroll_id=%s: %v", scrollId, marshalErr)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		req := opensearchapi.ScrollRequest{
			Body: bytes.NewReader(jsonBody),
		}
		res, err = req.Do(c.Request.Context(), s.osClient)
	} else {
		// Use Body for sort and scroll to minimize URL parameters
		searchBody := map[string]any{
			"sort": []map[string]any{
				{"indexedAt": "desc"},
			},
		}
		jsonBody, marshalErr := json.Marshal(searchBody)
		if marshalErr != nil {
			log.Errorf("unable to marshal search body: %v", marshalErr)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		req := opensearchapi.SearchRequest{
			Index:  []string{s.osIndex},
			Body:   bytes.NewReader(jsonBody),
			Scroll: 10 * time.Minute,
		}
		res, err = req.Do(c.Request.Context(), s.osClient)
	}
	if err != nil {
		log.Errorf("unable to get documents (scroll=%v): %v", scrollId != "", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if res.IsError() {
		log.Warnf("unable to get documents (scroll=%v): %s", scrollId != "", res.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	var docs struct {
		Hits struct {
			Hits []Document[models.Document] `json:"hits"`
		} `json:"hits"`
		ScrollId string `json:"_scroll_id"`
	}
	err = json.NewDecoder(res.Body).Decode(&docs)
	if err != nil {
		log.Errorf("unable to decode documents response (scroll=%v): %v", scrollId != "", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.JSON(http.StatusOK, docs)
}

func (s *Server) pingOs(ctx context.Context) error {
	req := opensearchapi.PingRequest{}
	res, err := req.Do(ctx, s.osClient)
	if err != nil {
		return fmt.Errorf("ping OpenSearch: %w", err)
	}
	if res.IsError() {
		return fmt.Errorf("ping OpenSearch returned %s", res.Status())
	}
	return nil
}

func (s *Server) verifyIndex(ctx context.Context, index string) error {
	req := opensearchapi.CatIndicesRequest{Index: []string{index}}
	res, err := req.Do(ctx, s.osClient)
	if err != nil {
		return fmt.Errorf("list index %s: %w", index, err)
	}

	if res.IsError() {
		return fmt.Errorf("unable to verify index %s: %s", index, res.Status())
	}
	return nil
}
