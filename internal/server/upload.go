package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"
	"time"

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
	sniffBufferSize  = 512       // bytes used for MIME sniffing
)

// allowedUploadMimeTypes enumerates the MIME types accepted by the upload
// endpoint. Detected via net/http.DetectContentType. Exposed as a package-level
// var so it can be extended without modifying the handler logic.
var allowedUploadMimeTypes = map[string]struct{}{
	"image/png":       {},
	"image/jpeg":      {},
	"image/webp":      {},
	"image/heic":      {},
	"image/heif":      {},
	"image/tiff":      {},
	"application/pdf": {},
}

// User-facing error strings — kept opaque so internal subsystem names don't
// leak into HTTP responses. The corresponding detail is always logged.
const (
	userErrUploadFailed     = "upload failed"
	userErrDuplicateDoc     = "duplicate document"
	userErrProcessingFailed = "document processing failed"
	userErrUnsupportedMedia = "unsupported media type"
)

// isAllowedMime returns true when the detected MIME type is in the allow-list.
func isAllowedMime(mime string) bool {
	_, ok := allowedUploadMimeTypes[mime]
	return ok
}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}

	defer func() { _ = form.RemoveAll() }()
	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files provided"})
		return
	}

	scanID := c.PostForm("scanID")
	if scanID == "" {
		scanID = uuid.NewString()
	} else if _, err := uuid.Parse(scanID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scanID"})
		return
	}
	sequenceIDs, err := parseUploadSequenceIDs(c.PostForm("sequenceIDs"), c.PostForm("sequenceOffset"), len(files))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page sequence"})
		return
	}
	log.Infof("upload started: scan=%s files=%d", scanID, len(files))
	results := make([]uploadPageResult, len(files))

	// Pre-scan every file so we can reject the whole request with 415 before
	// performing any storage work if even one file is of a disallowed type.
	type validatedFile struct {
		fh   *multipart.FileHeader
		mime string
	}
	validated := make([]validatedFile, len(files))
	for i, fh := range files {
		f, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to open uploaded file"})
			log.Errorf("upload scan=%s page=%d: unable to open uploaded file for mime sniff: %v", scanID, sequenceIDs[i], err)
			return
		}
		buf := make([]byte, sniffBufferSize)
		n, err := io.ReadFull(f, buf)
		_ = f.Close()
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read uploaded file"})
			log.Errorf("upload scan=%s page=%d: unable to read uploaded file for mime sniff: %v", scanID, sequenceIDs[i], err)
			return
		}
		mime := http.DetectContentType(buf[:n])
		if idx := bytes.IndexByte([]byte(mime), ';'); idx >= 0 {
			mime = mime[:idx]
		}
		if !isAllowedMime(mime) {
			log.Warnf("upload scan=%s page=%d: rejecting file with disallowed mime %q", scanID, sequenceIDs[i], mime)
			c.JSON(http.StatusUnsupportedMediaType, gin.H{
				"error":    userErrUnsupportedMedia,
				"filename": fh.Filename,
				"detected": mime,
			})
			return
		}
		validated[i] = validatedFile{fh: fh, mime: mime}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	sem := make(chan struct{}, uploadMaxWorkers)

	for i, vf := range validated {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, fileHeader *multipart.FileHeader, detectedMime string) {
			defer wg.Done()
			defer func() { <-sem }()

			result := uploadPageResult{
				SequenceID: sequenceIDs[idx],
			}

			f, err := fileHeader.Open()
			if err != nil {
				result.Status = "failed"
				result.Error = userErrUploadFailed
				log.Errorf("upload scan=%s page=%d: unable to open uploaded file: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			defer func() { _ = f.Close() }()

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, f); err != nil {
				result.Status = "failed"
				result.Error = userErrUploadFailed
				log.Errorf("upload scan=%s page=%d: unable to read uploaded file: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

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
				result.Error = userErrUploadFailed
				log.Errorf("upload scan=%s page=%d: dedupe reservation failed: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			if !reservation.Reserved {
				result.Status = "duplicate"
				result.DuplicateOf = reservation.ExistingDocumentID
				result.Error = userErrDuplicateDoc
				log.Infof("upload scan=%s page=%d: duplicate of %s", scanID, result.SequenceID, reservation.ExistingDocumentID)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			if err := s.storage.Store(c.Request.Context(), page); err != nil {
				if releaseErr := s.releaseUploadDigest(c.Request.Context(), digest, page.ID()); releaseErr != nil {
					log.Warnf("upload scan=%s page=%d: unable to release content digest after storage failure: %v", scanID, result.SequenceID, releaseErr)
				}
				result.Status = "failed"
				result.Error = userErrUploadFailed
				log.Errorf("upload scan=%s page=%d: storage failure: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}

			if _, err := reader.Seek(0, io.SeekStart); err != nil {
				result.Status = "failed"
				result.Error = userErrUploadFailed
				log.Errorf("upload scan=%s page=%d: unable to seek after storage: %v", scanID, result.SequenceID, err)
				mu.Lock()
				results[idx] = result
				mu.Unlock()
				return
			}
			page.Reader = reader

			if err := s.indexer.Index(c.Request.Context(), page); err != nil {
				if releaseErr := s.releaseUploadDigest(c.Request.Context(), digest, page.ID()); releaseErr != nil {
					log.Warnf("upload scan=%s page=%d: unable to release content digest after index failure: %v", scanID, result.SequenceID, releaseErr)
				}
				result.Status = "failed"
				result.Error = userErrProcessingFailed
				log.Errorf("upload scan=%s page=%d: indexer failure: %v", scanID, result.SequenceID, err)
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
			_ = detectedMime
		}(i, vf.fh, vf.mime)
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

	// If every page was a duplicate, surface that as a 409 Conflict so clients
	// can distinguish a fully-duplicate batch from a partial-success upload.
	status := http.StatusOK
	if processed == 0 && failed == 0 && duplicates > 0 {
		status = http.StatusConflict
	}

	c.JSON(status, uploadResponse{
		ScanID:     scanID,
		Processed:  processed,
		Duplicates: duplicates,
		Failed:     failed,
		Pages:      results,
	})
}

// Explicit IDs preserve scanner ordering even when concurrent workers complete out of order.
// Older clients can continue sending a contiguous sequenceOffset.
func parseUploadSequenceIDs(rawIDs, rawOffset string, count int) ([]int, error) {
	ids := make([]int, count)
	if rawIDs != "" {
		if err := json.Unmarshal([]byte(rawIDs), &ids); err != nil || len(ids) != count {
			return nil, errors.New("sequenceIDs must match files")
		}
	} else {
		offset := 0
		if rawOffset != "" {
			var err error
			offset, err = strconv.Atoi(rawOffset)
			if err != nil || offset < 0 || offset > math.MaxInt-count {
				return nil, errors.New("invalid sequenceOffset")
			}
		}
		for i := range ids {
			ids[i] = offset + i + 1
		}
	}
	seen := make(map[int]bool, count)
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return nil, errors.New("page IDs must be positive and unique")
		}
		seen[id] = true
	}
	return ids, nil
}

// Cleanup must still run when an upload client disconnects.
func (s *Server) releaseUploadDigest(ctx context.Context, digest, id string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return s.indexer.ReleaseContentDigest(cleanupCtx, digest, id)
}
