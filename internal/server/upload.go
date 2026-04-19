package server

import (
	"bytes"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/denysvitali/odi/pkg/models"
)

type uploadPageResult struct {
	SequenceID int    `json:"sequenceID"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type uploadResponse struct {
	ScanID    string             `json:"scanID"`
	Processed int                `json:"processed"`
	Failed    int                `json:"failed"`
	Pages     []uploadPageResult `json:"pages"`
}

const maxUploadSize = 200 << 20 // 200 MB total

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

	scanID := uuid.NewString()
	results := make([]uploadPageResult, len(files))

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, fh := range files {
		wg.Add(1)
		go func(idx int, filename string) {
			defer wg.Done()

			result := uploadPageResult{
				SequenceID: idx + 1,
			}

			f, err := fh.Open()
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
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
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			f.Close()

			reader := bytes.NewReader(buf.Bytes())
			page := models.ScannedPage{
				Reader:     reader,
				ScanID:     scanID,
				SequenceID: idx + 1,
			}

			if err := s.storage.Store(c.Request.Context(), page); err != nil {
				result.Status = "failed"
				result.Error = "storage: " + err.Error()
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			reader.Seek(0, io.SeekStart)
			page.Reader = reader

			if err := s.indexer.Index(c.Request.Context(), page); err != nil {
				result.Status = "failed"
				result.Error = "index: " + err.Error()
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			result.Status = "indexed"
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, fh.Filename)
	}

	wg.Wait()

	processed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "indexed" {
			processed++
		} else {
			failed++
		}
	}

	c.JSON(http.StatusOK, uploadResponse{
		ScanID:    scanID,
		Processed: processed,
		Failed:    failed,
		Pages:     results,
	})
}
