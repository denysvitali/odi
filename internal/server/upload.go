package server

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/denysvitali/odi/pkg/contentdigest"
	"github.com/denysvitali/odi/pkg/models"
)

type uploadPageResult struct {
	SequenceID  int    `json:"sequenceID"`
	Status      string `json:"status"`
	DuplicateOf string `json:"duplicateOf,omitempty"`
	Error       string `json:"error,omitempty"`
}

type uploadResponse struct {
	ScanID     string             `json:"scanID"`
	Processed  int                `json:"processed"`
	Duplicates int                `json:"duplicates"`
	Failed     int                `json:"failed"`
	Pages      []uploadPageResult `json:"pages"`
}

const (
	maxUploadSize    = 200 << 20 // 200 MB total
	uploadMaxWorkers = 8         // cap per-upload in-flight file processing
)

func (s *Server) handleUpload(c *gin.Context) {
	if s.indexer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "upload endpoint not configured: OCR/Indexer not initialized",
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form: " + err.Error()})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files provided"})
		return
	}

	scanID := c.PostForm("scanID")
	if scanID == "" {
		scanID = uuid.NewString()
	} else if _, err := uuid.Parse(scanID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scanID: " + err.Error()})
		return
	}
	sequenceOffset := 0
	if rawOffset := c.PostForm("sequenceOffset"); rawOffset != "" {
		parsedOffset, err := strconv.Atoi(rawOffset)
		if err != nil || parsedOffset < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sequenceOffset"})
			return
		}
		sequenceOffset = parsedOffset
	}
	log.Infof("upload started: scan=%s files=%d", scanID, len(files))
	results := make([]uploadPageResult, len(files))

	var wg sync.WaitGroup
	var mu sync.Mutex

	sem := make(chan struct{}, uploadMaxWorkers)

	for i, fh := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, fileHeader *multipart.FileHeader) {
			defer wg.Done()
			defer func() { <-sem }()

			result := uploadPageResult{
				SequenceID: sequenceOffset + idx + 1,
			}

			f, err := fileHeader.Open()
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				log.Errorf("upload scan=%s page=%d: unable to open uploaded file %q: %v", scanID, result.SequenceID, fileHeader.Filename, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, f); err != nil {
				f.Close()
				result.Status = "failed"
				result.Error = err.Error()
				log.Errorf("upload scan=%s page=%d: unable to read uploaded file %q: %v", scanID, result.SequenceID, fileHeader.Filename, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			f.Close()

			digest := contentdigest.Sum(buf.Bytes())
			reader := bytes.NewReader(buf.Bytes())
			page := models.ScannedPage{
				Reader:        reader,
				ScanID:        scanID,
				SequenceID:    result.SequenceID,
				ContentDigest: digest,
			}

			reservation, err := s.indexer.ReserveContentDigest(c.Request.Context(), digest, page.ID())
			if err != nil {
				result.Status = "failed"
				result.Error = "dedupe: " + err.Error()
				log.Errorf("upload scan=%s page=%d: unable to reserve content digest: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			if !reservation.Reserved {
				result.Status = "duplicate"
				result.DuplicateOf = reservation.ExistingDocumentID
				log.Infof("upload scan=%s page=%d: duplicate of %s", scanID, result.SequenceID, reservation.ExistingDocumentID)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			if err := s.storage.Store(c.Request.Context(), page); err != nil {
				if releaseErr := s.indexer.ReleaseContentDigest(c.Request.Context(), digest, page.ID()); releaseErr != nil {
					log.Warnf("upload scan=%s page=%d: unable to release content digest after storage failure: %v", scanID, result.SequenceID, releaseErr)
				}
				result.Status = "failed"
				result.Error = "storage: " + err.Error()
				log.Errorf("upload scan=%s page=%d: unable to store page: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			if _, err := reader.Seek(0, io.SeekStart); err != nil {
				result.Status = "failed"
				result.Error = "seek: " + err.Error()
				log.Errorf("upload scan=%s page=%d: unable to seek after storage: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			page.Reader = reader

			if err := s.indexer.Index(c.Request.Context(), page); err != nil {
				if releaseErr := s.indexer.ReleaseContentDigest(c.Request.Context(), digest, page.ID()); releaseErr != nil {
					log.Warnf("upload scan=%s page=%d: unable to release content digest after index failure: %v", scanID, result.SequenceID, releaseErr)
				}
				result.Status = "failed"
				result.Error = "index: " + err.Error()
				log.Errorf("upload scan=%s page=%d: unable to index page: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			if generated, _, err := s.ensureThumbnailFromReader(c.Request.Context(), page.ScanID, page.SequenceID, bytes.NewReader(buf.Bytes())); err != nil {
				if !errors.Is(err, errThumbnailStorageUnsupported) {
					log.Warnf("upload scan=%s page=%d: unable to generate thumbnail: %v", scanID, result.SequenceID, err)
				}
			} else if generated {
				log.Debugf("upload scan=%s page=%d: generated thumbnail", scanID, result.SequenceID)
			}

			result.Status = "indexed"
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, fh)
	}

	wg.Wait()

	processed := 0
	duplicates := 0
	failed := 0
	for _, r := range results {
		switch r.Status {
		case "indexed":
			processed++
		case "duplicate":
			duplicates++
		default:
			failed++
		}
	}

	log.Infof("upload finished: scan=%s processed=%d duplicates=%d failed=%d", scanID, processed, duplicates, failed)
	c.JSON(http.StatusOK, uploadResponse{
		ScanID:     scanID,
		Processed:  processed,
		Duplicates: duplicates,
		Failed:     failed,
		Pages:      results,
	})
}
