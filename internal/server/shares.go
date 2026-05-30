package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"golang.org/x/crypto/bcrypt"
)

// sharesIndex is the OpenSearch index that stores share-link records. It is
// created on first use (create-if-missing) so no migration step is required.
const sharesIndex = "odi-shares"

// shareRecord is the document stored in sharesIndex. The token is also used as
// the OpenSearch document ID so lookups by token are O(1) GETs.
type shareRecord struct {
	Token          string `json:"token"`
	ScanID         string `json:"scanID"`
	SequenceID     int    `json:"sequenceID"`
	ExpiresAt      int64  `json:"expiresAt"`
	MaxViews       int    `json:"maxViews"`
	ViewCount      int    `json:"viewCount"`
	PassphraseHash string `json:"passphraseHash,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
	Revoked        bool   `json:"revoked"`
}

// createShareRequest is the JSON body accepted by handleCreateShare.
type createShareRequest struct {
	ScanID         string `json:"scanID" binding:"required"`
	SequenceID     int    `json:"sequenceID"`
	ExpiresInHours int    `json:"expiresInHours"`
	MaxViews       int    `json:"maxViews"`
	Passphrase     string `json:"passphrase,omitempty"`
}

// shareView is the public projection of a share record (passphraseHash is
// never leaked to clients).
type shareView struct {
	Token      string `json:"token"`
	ScanID     string `json:"scanID"`
	SequenceID int    `json:"sequenceID"`
	ExpiresAt  int64  `json:"expiresAt"`
	MaxViews   int    `json:"maxViews"`
	ViewCount  int    `json:"viewCount"`
	CreatedAt  int64  `json:"createdAt"`
	HasPass    bool   `json:"hasPassphrase"`
}

const defaultShareExpiryHours = 24

// ensureSharesIndex creates sharesIndex if it does not already exist, mirroring
// the no-op-if-exists style of indexer.createIndex. A dynamic mapping is fine
// for these records, so no explicit mapping body is supplied.
func (s *Server) ensureSharesIndex(ctx context.Context) error {
	resp, err := s.osClient.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{Indices: []string{sharesIndex}})
	if err == nil && resp != nil && resp.StatusCode < 400 {
		resp.Body.Close()
		return nil
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("check shares index: %s", resp.Status())
		}
	}

	createResp, err := s.osClient.Indices.Create(ctx, opensearchapi.IndicesCreateReq{Index: sharesIndex})
	if err != nil {
		return fmt.Errorf("create shares index: %w", err)
	}
	defer createResp.Inspect().Response.Body.Close()

	statusCode := createResp.Inspect().Response.StatusCode
	if statusCode == http.StatusOK || statusCode == http.StatusCreated {
		return nil
	}
	if statusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(createResp.Inspect().Response.Body)
		if strings.Contains(string(body), "resource_already_exists_exception") {
			return nil
		}
		return fmt.Errorf("create shares index returned %s: %s", createResp.Inspect().Response.Status(), string(body))
	}
	return fmt.Errorf("create shares index: unexpected status %s", createResp.Inspect().Response.Status())
}

// handleCreateShare mints a new HMAC-signed share token for a single page and
// persists a record for revocation / view-count tracking. Requires API_TOKEN.
func (s *Server) handleCreateShare(c *gin.Context) {
	if s.apiToken == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sharing requires API_TOKEN"})
		return
	}

	var req createShareRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}
	if req.SequenceID < 0 {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	hours := req.ExpiresInHours
	if hours <= 0 {
		hours = defaultShareExpiryHours
	}
	expiresAt := time.Now().Add(time.Duration(hours) * time.Hour).Unix()

	maxViews := req.MaxViews
	if maxViews < 0 {
		maxViews = 0
	}

	payload := SharePayload{
		ScanID:     req.ScanID,
		SequenceID: req.SequenceID,
		ExpiresAt:  expiresAt,
		MaxViews:   maxViews,
	}

	token, err := signShare([]byte(s.apiToken), payload)
	if err != nil {
		log.Errorf("unable to sign share token (scan=%s seq=%d): %v", req.ScanID, req.SequenceID, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	var passHash string
	if req.Passphrase != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Passphrase), bcrypt.DefaultCost)
		if err != nil {
			log.Errorf("unable to hash share passphrase: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
			return
		}
		passHash = string(h)
	}

	ctx := c.Request.Context()
	if err := s.ensureSharesIndex(ctx); err != nil {
		log.Errorf("unable to ensure shares index: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	record := shareRecord{
		Token:          token,
		ScanID:         req.ScanID,
		SequenceID:     req.SequenceID,
		ExpiresAt:      expiresAt,
		MaxViews:       maxViews,
		ViewCount:      0,
		PassphraseHash: passHash,
		CreatedAt:      time.Now().Unix(),
		Revoked:        false,
	}

	body, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		log.Errorf("unable to marshal share record: %v", marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	createResp, err := s.osClient.Document.Create(ctx, opensearchapi.DocumentCreateReq{
		Index:      sharesIndex,
		DocumentID: token,
		Body:       bytes.NewReader(body),
		Params:     opensearchapi.DocumentCreateParams{Refresh: "true"},
	})
	if err != nil {
		log.Errorf("unable to store share record (scan=%s seq=%d): %v", req.ScanID, req.SequenceID, err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer createResp.Inspect().Response.Body.Close()
	if createResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("store share record returned %s", createResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"urlSuffix": "/share/" + token,
		"expiresAt": expiresAt,
	})
}

// handleListShares returns the active (not revoked, not expired) share records,
// omitting passphrase hashes.
func (s *Server) handleListShares(c *gin.Context) {
	ctx := c.Request.Context()
	if err := s.ensureSharesIndex(ctx); err != nil {
		log.Errorf("unable to ensure shares index: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	now := time.Now().Unix()
	searchBody := map[string]any{
		"size": maxSearchSize,
		"sort": []map[string]any{{"createdAt": "desc"}},
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []map[string]any{
					{"term": map[string]any{"revoked": false}},
					{"range": map[string]any{"expiresAt": map[string]any{"gt": now}}},
				},
			},
		},
	}

	jsonBody, marshalErr := json.Marshal(searchBody)
	if marshalErr != nil {
		log.Errorf("unable to marshal list-shares body: %v", marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	searchResp, err := s.osClient.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{sharesIndex},
		Body:    bytes.NewReader(jsonBody),
	})
	if err != nil {
		log.Errorf("unable to list shares: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer searchResp.Inspect().Response.Body.Close()
	if searchResp.Inspect().Response.StatusCode >= 400 {
		log.Errorf("list shares returned %s", searchResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	var parsed struct {
		Hits struct {
			Hits []struct {
				Source shareRecord `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(searchResp.Inspect().Response.Body).Decode(&parsed); err != nil {
		log.Errorf("unable to decode list-shares response: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	views := make([]shareView, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		r := h.Source
		views = append(views, shareView{
			Token:      r.Token,
			ScanID:     r.ScanID,
			SequenceID: r.SequenceID,
			ExpiresAt:  r.ExpiresAt,
			MaxViews:   r.MaxViews,
			ViewCount:  r.ViewCount,
			CreatedAt:  r.CreatedAt,
			HasPass:    r.PassphraseHash != "",
		})
	}

	c.JSON(http.StatusOK, gin.H{"shares": views})
}

// handleRevokeShare flips revoked=true on the record identified by the :token
// path parameter.
func (s *Server) handleRevokeShare(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	ctx := c.Request.Context()
	updateBody, marshalErr := json.Marshal(map[string]any{
		"doc": map[string]any{"revoked": true},
	})
	if marshalErr != nil {
		log.Errorf("unable to marshal revoke body: %v", marshalErr)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	updateResp, err := s.osClient.Update(ctx, opensearchapi.UpdateReq{
		Index:      sharesIndex,
		DocumentID: token,
		Body:       bytes.NewReader(updateBody),
		Params:     opensearchapi.UpdateParams{Refresh: "true"},
	})
	if err != nil {
		log.Errorf("unable to revoke share: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	defer updateResp.Inspect().Response.Body.Close()

	statusCode := updateResp.Inspect().Response.StatusCode
	if statusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
		return
	}
	if statusCode >= 400 {
		log.Errorf("revoke share returned %s", updateResp.Inspect().Response.Status())
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// loadShareRecord fetches a share record by token. Returns (record, found, err).
func (s *Server) loadShareRecord(ctx context.Context, token string) (shareRecord, bool, error) {
	var rec shareRecord
	getResp, err := s.osClient.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index:      sharesIndex,
		DocumentID: token,
	})
	if err != nil {
		if getResp != nil && getResp.Inspect().Response.StatusCode == http.StatusNotFound {
			return rec, false, nil
		}
		return rec, false, fmt.Errorf("get share record: %w", err)
	}
	statusCode := getResp.Inspect().Response.StatusCode
	if statusCode == http.StatusNotFound {
		return rec, false, nil
	}
	if statusCode >= 400 {
		return rec, false, fmt.Errorf("get share record returned %s", getResp.Inspect().Response.Status())
	}
	if err := json.Unmarshal(getResp.Source, &rec); err != nil {
		return rec, false, fmt.Errorf("decode share record: %w", err)
	}
	return rec, true, nil
}

// handleServeShare is the PUBLIC endpoint that resolves a share token and
// streams the underlying page. All rejection paths return a plain 404 so the
// endpoint never leaks whether a given token exists, is revoked, or expired.
func (s *Server) handleServeShare(c *gin.Context) {
	notFound := func() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	}

	if s.apiToken == "" {
		// Without a secret we cannot verify any token; treat as not found.
		notFound()
		return
	}

	token := c.Param("token")
	if token == "" {
		notFound()
		return
	}

	payload, err := verifyShare([]byte(s.apiToken), token)
	if err != nil {
		log.Debugf("share token verification failed: %v", err)
		notFound()
		return
	}

	ctx := c.Request.Context()
	rec, found, err := s.loadShareRecord(ctx, token)
	if err != nil {
		log.Errorf("unable to load share record: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	if !found {
		notFound()
		return
	}

	now := time.Now().Unix()
	if rec.Revoked || rec.ExpiresAt <= now || payload.ExpiresAt <= now {
		notFound()
		return
	}
	if rec.MaxViews > 0 && rec.ViewCount >= rec.MaxViews {
		notFound()
		return
	}

	if rec.PassphraseHash != "" {
		provided := c.Query("p")
		if provided == "" {
			provided = c.GetHeader("X-Share-Passphrase")
		}
		if provided == "" {
			notFound()
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(rec.PassphraseHash), []byte(provided)); err != nil {
			// Constant-time-ish comparison via bcrypt; reject without leaking.
			_ = subtle.ConstantTimeCompare([]byte(provided), []byte(provided))
			notFound()
			return
		}
	}

	// Increment the view count before streaming. A failure here is logged but
	// does not block delivery of the page.
	if err := s.incrementShareViews(ctx, token); err != nil {
		log.Warnf("unable to increment share view count for token: %v", err)
	}

	s.returnDocument(c, payload.ScanID, fmt.Sprint(payload.SequenceID))
}

// incrementShareViews atomically bumps viewCount via a painless script update.
func (s *Server) incrementShareViews(ctx context.Context, token string) error {
	updateBody, err := json.Marshal(map[string]any{
		"script": map[string]any{
			"source": "ctx._source.viewCount += 1",
			"lang":   "painless",
		},
	})
	if err != nil {
		return fmt.Errorf("marshal increment body: %w", err)
	}

	updateResp, err := s.osClient.Update(ctx, opensearchapi.UpdateReq{
		Index:      sharesIndex,
		DocumentID: token,
		Body:       bytes.NewReader(updateBody),
		Params:     opensearchapi.UpdateParams{Refresh: "true"},
	})
	if err != nil {
		return fmt.Errorf("increment share views: %w", err)
	}
	defer updateResp.Inspect().Response.Body.Close()
	if updateResp.Inspect().Response.StatusCode >= 400 {
		return errors.New("increment share views returned " + updateResp.Inspect().Response.Status())
	}
	return nil
}
