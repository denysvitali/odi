package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/denysvitali/odi/pkg/models"
	"github.com/denysvitali/odi/pkg/storage/model"
	"github.com/denysvitali/odi/pkg/thumbnailer"
)

const (
	thumbnailProcessingInterval = time.Hour
	thumbnailProcessingBatch    = 500
)

var (
	errThumbnailStorageUnsupported = errors.New("storage does not support thumbnails")
	errThumbnailProcessingRunning  = errors.New("thumbnail processing is already running")
)

type thumbnailProcessingResult struct {
	Checked   int `json:"checked"`
	Existing  int `json:"existing"`
	Generated int `json:"generated"`
	Failed    int `json:"failed"`
}

type thumbnailSearchResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []struct {
			Source models.Document `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func (s *Server) handleProcessMissingThumbnails(c *gin.Context) {
	result, err := s.ProcessMissingThumbnails(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, errThumbnailStorageUnsupported):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		case errors.Is(err, errThumbnailProcessingRunning):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			log.Errorf("thumbnail processing failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) startThumbnailProcessor(ctx context.Context) {
	if _, ok := s.storage.(model.ThumbnailStorage); !ok {
		return
	}

	go func() {
		ticker := time.NewTicker(thumbnailProcessingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := s.ProcessMissingThumbnails(ctx)
				if err != nil {
					if !errors.Is(err, errThumbnailProcessingRunning) {
						log.Warnf("scheduled thumbnail processing failed: %v", err)
					}
					continue
				}
				log.Infof("scheduled thumbnail processing complete: checked=%d existing=%d generated=%d failed=%d",
					result.Checked, result.Existing, result.Generated, result.Failed)
			}
		}
	}()
}

func (s *Server) ProcessMissingThumbnails(ctx context.Context) (thumbnailProcessingResult, error) {
	if _, ok := s.storage.(model.ThumbnailStorage); !ok {
		return thumbnailProcessingResult{}, errThumbnailStorageUnsupported
	}
	if !s.thumbnailProcessMu.TryLock() {
		return thumbnailProcessingResult{}, errThumbnailProcessingRunning
	}
	defer s.thumbnailProcessMu.Unlock()

	return s.processMissingThumbnails(ctx)
}

func (s *Server) processMissingThumbnails(ctx context.Context) (thumbnailProcessingResult, error) {
	var result thumbnailProcessingResult

	searchBody := map[string]any{
		"size":    thumbnailProcessingBatch,
		"_source": []string{"scanID", "sequenceID"},
		"query":   map[string]any{"match_all": map[string]any{}},
	}
	body, err := json.Marshal(searchBody)
	if err != nil {
		return result, fmt.Errorf("marshal thumbnail processing search: %w", err)
	}

	searchResp, err := s.osClient.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(body),
		Params: opensearchapi.SearchParams{
			Scroll: 5 * time.Minute,
		},
	})
	if err != nil {
		return result, fmt.Errorf("search documents for thumbnail processing: %w", err)
	}
	defer searchResp.Inspect().Response.Body.Close()
	if searchResp.Inspect().Response.StatusCode >= 400 {
		return result, fmt.Errorf("search documents for thumbnail processing returned %s", searchResp.Inspect().Response.Status())
	}

	var decoded thumbnailSearchResponse
	if err := json.NewDecoder(searchResp.Inspect().Response.Body).Decode(&decoded); err != nil {
		return result, fmt.Errorf("decode thumbnail processing search response: %w", err)
	}
	scrollID := decoded.ScrollID
	defer s.clearScroll(ctx, scrollID)

	for {
		if len(decoded.Hits.Hits) == 0 {
			return result, nil
		}

		s.processThumbnailHits(ctx, decoded.Hits.Hits, &result)

		scrollResp, err := s.osClient.Scroll.Get(ctx, opensearchapi.ScrollGetReq{
			ScrollID: scrollID,
			Params: opensearchapi.ScrollGetParams{
				Scroll: 5 * time.Minute,
			},
		})
		if err != nil {
			return result, fmt.Errorf("continue thumbnail processing scroll: %w", err)
		}
		if scrollResp.Inspect().Response.StatusCode >= 400 {
			status := scrollResp.Inspect().Response.Status()
			scrollResp.Inspect().Response.Body.Close()
			return result, fmt.Errorf("continue thumbnail processing scroll returned %s", status)
		}

		decoded = thumbnailSearchResponse{}
		if err := json.NewDecoder(scrollResp.Inspect().Response.Body).Decode(&decoded); err != nil {
			scrollResp.Inspect().Response.Body.Close()
			return result, fmt.Errorf("decode thumbnail processing scroll response: %w", err)
		}
		scrollResp.Inspect().Response.Body.Close()
		if decoded.ScrollID != "" {
			scrollID = decoded.ScrollID
		}
	}
}

func (s *Server) processThumbnailHits(ctx context.Context, hits []struct {
	Source models.Document `json:"_source"`
}, result *thumbnailProcessingResult) {
	for _, hit := range hits {
		doc := hit.Source
		if doc.ScanID == "" || doc.SequenceID <= 0 {
			result.Failed++
			continue
		}

		generated, existing, err := s.ensureStoredThumbnail(ctx, doc.ScanID, doc.SequenceID)
		result.Checked++
		switch {
		case err != nil:
			result.Failed++
			log.Warnf("thumbnail processing failed for %s_%d: %v", doc.ScanID, doc.SequenceID, err)
		case existing:
			result.Existing++
		case generated:
			result.Generated++
		}
	}
}

func (s *Server) ensureStoredThumbnail(ctx context.Context, scanID string, sequenceID int) (generated bool, existing bool, err error) {
	ts, ok := s.storage.(model.ThumbnailStorage)
	if !ok {
		return false, false, errThumbnailStorageUnsupported
	}

	exists, err := ts.ThumbnailExists(ctx, scanID, sequenceID)
	if err != nil {
		return false, false, fmt.Errorf("check thumbnail: %w", err)
	}
	if exists {
		return false, true, nil
	}

	page, err := s.storage.Retrieve(ctx, scanID, sequenceID)
	if err != nil {
		return false, false, fmt.Errorf("retrieve page: %w", err)
	}
	if _, err := page.Reader.Seek(0, io.SeekStart); err != nil {
		return false, false, fmt.Errorf("seek page: %w", err)
	}

	return s.storeThumbnailFromReader(ctx, scanID, sequenceID, page.Reader)
}

func (s *Server) ensureThumbnailFromReader(ctx context.Context, scanID string, sequenceID int, reader io.Reader) (generated bool, existing bool, err error) {
	ts, ok := s.storage.(model.ThumbnailStorage)
	if !ok {
		return false, false, errThumbnailStorageUnsupported
	}

	exists, err := ts.ThumbnailExists(ctx, scanID, sequenceID)
	if err != nil {
		return false, false, fmt.Errorf("check thumbnail: %w", err)
	}
	if exists {
		return false, true, nil
	}

	return s.storeThumbnailFromReader(ctx, scanID, sequenceID, reader)
}

func (s *Server) storeThumbnailFromReader(ctx context.Context, scanID string, sequenceID int, reader io.Reader) (bool, bool, error) {
	ts, ok := s.storage.(model.ThumbnailStorage)
	if !ok {
		return false, false, errThumbnailStorageUnsupported
	}

	thumbReader, err := thumbnailer.Generate(reader)
	if err != nil {
		return false, false, fmt.Errorf("generate thumbnail: %w", err)
	}
	if err := ts.StoreThumbnail(ctx, scanID, sequenceID, thumbReader); err != nil {
		return false, false, fmt.Errorf("store thumbnail: %w", err)
	}
	return true, false, nil
}

func (s *Server) clearScroll(ctx context.Context, scrollID string) {
	if scrollID == "" {
		return
	}
	resp, err := s.osClient.Scroll.Delete(ctx, opensearchapi.ScrollDeleteReq{ScrollIDs: []string{scrollID}})
	if err != nil {
		log.Debugf("unable to clear thumbnail processing scroll: %v", err)
		return
	}
	if resp != nil && resp.Inspect().Response != nil && resp.Inspect().Response.Body != nil {
		resp.Inspect().Response.Body.Close()
	}
}
