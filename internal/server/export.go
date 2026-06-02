package server

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"golang.org/x/sync/errgroup"

	odicrypt "github.com/denysvitali/odi/pkg/crypt"
	"github.com/denysvitali/odi/pkg/storage/model"
)

// exportScrollSize is the page size used while scrolling the matching documents
// for an export. Each page is fetched, its files streamed into the archive, and
// then the next page requested until the scroll is exhausted.
const exportScrollSize = 200

// exportScrollTTL is how long OpenSearch keeps the scroll context alive between
// pages. It is generous because streaming many files into the zip can take a
// while for large bundles.
const exportScrollTTL = 5 * time.Minute

// exportRetrieveConcurrency bounds how many storage retrievals run in parallel
// while collecting a page. For a network backend (B2) each Retrieve is a
// synchronous round-trip, so fetching pages concurrently turns N x latency into
// roughly N/concurrency x latency. The retrieved bytes are then written into the
// zip serially (zip.Writer is not safe for concurrent writes), so this only
// parallelizes the network I/O, not the archive writing.
const exportRetrieveConcurrency = 8

// exportMaxDocuments caps how many documents a single export may include.
//
// AES-GCM (pkg/crypt) is not a streaming mode: handleExport builds the entire
// plaintext zip in memory and then encrypts the whole buffer in one shot, so the
// plaintext bundle plus its ciphertext must fit in RAM. pkg/crypt deliberately
// does not expose a chunked/streaming encrypt API, and inventing a new on-the-wire
// crypto format here would be risky and out of scope. Instead we enforce this
// hard cap so an oversized export fails cleanly with a 413 rather than OOMing the
// server. With typical scanned pages this keeps the in-memory bundle bounded well
// below pkg/crypt's own maxDecryptSize (200 MiB) ceiling.
const exportMaxDocuments = 5000

// ExportRequest carries the same structured filter fields as SearchRequest plus
// the passphrase that the resulting archive is encrypted under. The passphrase
// is never persisted: it is used in-process to derive the AES-256-GCM key and
// to HMAC the manifest, then discarded.
type ExportRequest struct {
	Companies  []string `json:"companies,omitempty"`
	DateFrom   string   `json:"dateFrom,omitempty"`
	DateTo     string   `json:"dateTo,omitempty"`
	HasBarcode *bool    `json:"hasBarcode,omitempty"`
	Title      string   `json:"title,omitempty"`
	DocTypes   []string `json:"docTypes,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Passphrase string   `json:"passphrase"`
}

// manifestEntry records one page included in the bundle. The contentDigest lets
// the recipient verify the decrypted file against what the index claimed.
type manifestEntry struct {
	ScanID        string `json:"scanID"`
	SequenceID    int    `json:"sequenceID"`
	ContentDigest string `json:"contentDigest,omitempty"`
	FileName      string `json:"fileName"`
}

// exportManifest is the manifest.json embedded in (and signed over) the archive.
type exportManifest struct {
	GeneratedAt   int64           `json:"generatedAt"`
	DocumentCount int             `json:"documentCount"`
	Entries       []manifestEntry `json:"entries"`
}

// exportHit is the minimal projection of an OpenSearch hit needed to locate the
// stored file for a document and to build its manifest entry.
type exportHit struct {
	ID     string `json:"_id"`
	Source struct {
		ScanID        string `json:"scanID"`
		SequenceID    int    `json:"sequenceID"`
		ContentDigest string `json:"contentDigest"`
	} `json:"_source"`
}

type exportScrollResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []exportHit `json:"hits"`
	} `json:"hits"`
}

// handleExport produces an AES-256-GCM-encrypted ZIP of every document matching
// the supplied filter, accompanied by an HMAC-SHA256-signed manifest. The whole
// bundle is encrypted under the request passphrase (via pkg/crypt, the same
// helpers the B2 storage backend uses) and streamed back as
// application/octet-stream with a .zip.enc filename.
//
// The HMAC is keyed by the server apiToken (reusing the signShare HMAC pattern)
// so a recipient who knows the passphrase can decrypt the bundle while the
// manifest's integrity remains attributable to this server.
func (s *Server) handleExport(c *gin.Context) {
	var req ExportRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, badRequest)
		return
	}

	if strings.TrimSpace(req.Passphrase) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "passphrase is required"})
		return
	}

	if s.apiToken == "" {
		// Without a server secret we cannot sign the manifest.
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "export requires API_TOKEN"})
		return
	}

	ctx := c.Request.Context()

	// Build the matching-document filter from the same helper the search
	// endpoints use, so an export is scoped identically to a search.
	filters := buildSearchFilters(SearchRequest{
		Companies:  req.Companies,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		HasBarcode: req.HasBarcode,
		Title:      req.Title,
		DocTypes:   req.DocTypes,
		Tags:       req.Tags,
	})

	var query map[string]any
	if len(filters) > 0 {
		query = map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		}
	} else {
		query = map[string]any{"match_all": map[string]any{}}
	}

	// Assemble the plaintext archive in memory first so we can HMAC the manifest
	// and encrypt the whole thing as a single sealed unit. AES-GCM (pkg/crypt)
	// is not a streaming mode, so the bundle must fit in memory regardless.
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)

	entries, err := s.collectExportEntries(ctx, query, zw)
	if err != nil {
		switch {
		case errors.Is(err, errExportTooLarge):
			log.Warnf("export rejected: too many matching documents (cap %d)", exportMaxDocuments)
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("export matches too many documents (limit is %d); narrow the filter", exportMaxDocuments),
			})
		case errors.Is(err, errExportSearchFailed):
			log.Errorf("export grounding search failed: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
		default:
			log.Errorf("unable to collect export entries: %v", err)
			c.JSON(http.StatusInternalServerError, internalServerError)
		}
		return
	}

	manifest := exportManifest{
		GeneratedAt:   time.Now().Unix(),
		DocumentCount: len(entries),
		Entries:       entries,
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.Errorf("unable to marshal export manifest: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	// Sign the manifest with HMAC-SHA256 keyed by the server secret, mirroring
	// the signShare pattern. The hex signature is written alongside the manifest
	// so a recipient can verify it independently of decryption.
	signature := signManifest([]byte(s.apiToken), manifestBytes)

	if err := writeArchiveFile(zw, "manifest.json", manifestBytes); err != nil {
		log.Errorf("unable to write manifest to archive: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	if err := writeArchiveFile(zw, "manifest.json.sig", []byte(signature)); err != nil {
		log.Errorf("unable to write manifest signature to archive: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	if err := zw.Close(); err != nil {
		log.Errorf("unable to finalize export archive: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	// Encrypt the finished archive under the request passphrase using the modern
	// V1 AES-256-GCM + PBKDF2 format from pkg/crypt.
	crypter, err := odicrypt.New(req.Passphrase)
	if err != nil {
		log.Errorf("unable to init crypter for export: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}
	encrypted, err := crypter.Encrypt(bytes.NewReader(archive.Bytes()))
	if err != nil {
		log.Errorf("unable to encrypt export archive: %v", err)
		c.JSON(http.StatusInternalServerError, internalServerError)
		return
	}

	fileName := fmt.Sprintf("odi-export-%d.zip.enc", manifest.GeneratedAt)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.Header("X-Manifest-Signature", signature)
	c.Header("X-Document-Count", fmt.Sprint(manifest.DocumentCount))
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, encrypted); err != nil {
		logStreamError(c, err, "unable to stream encrypted export bundle")
	}
}

// errExportSearchFailed is returned when an OpenSearch query for the export set
// responds with an error status.
var errExportSearchFailed = errors.New("export search failed")

// errExportTooLarge is returned when the matching document set exceeds
// exportMaxDocuments. The whole bundle must fit in memory (AES-GCM is not a
// streaming mode), so we refuse oversized exports rather than risk an OOM.
var errExportTooLarge = errors.New("export exceeds maximum document count")

// collectExportEntries scrolls the matching documents, streams each stored page
// into zw under files/<scanID>_<sequenceID>, and returns the manifest entries.
// Pages that cannot be retrieved from storage (e.g. already deleted) are skipped
// with a warning rather than failing the whole export.
func (s *Server) collectExportEntries(ctx context.Context, query map[string]any, zw *zip.Writer) ([]manifestEntry, error) {
	searchContent := map[string]any{
		"size":    exportScrollSize,
		"query":   query,
		"_source": []string{"scanID", "sequenceID", "contentDigest"},
	}
	jsonBody, err := json.Marshal(searchContent)
	if err != nil {
		return nil, err
	}

	searchResp, err := s.osClient.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{s.osIndex},
		Body:    bytes.NewReader(jsonBody),
		Params: opensearchapi.SearchParams{
			Scroll: exportScrollTTL,
		},
	})
	if err != nil {
		return nil, err
	}
	if searchResp.Inspect().Response.StatusCode >= 400 {
		searchResp.Inspect().Response.Body.Close()
		return nil, errExportSearchFailed
	}

	var parsed exportScrollResponse
	decodeErr := json.NewDecoder(searchResp.Inspect().Response.Body).Decode(&parsed)
	searchResp.Inspect().Response.Body.Close()
	if decodeErr != nil {
		return nil, decodeErr
	}

	scrollID := parsed.ScrollID
	// Best-effort cleanup of the scroll context once we are done paging.
	defer s.clearScroll(ctx, scrollID)

	entries := make([]manifestEntry, 0, len(parsed.Hits.Hits))
	total := 0

	for len(parsed.Hits.Hits) > 0 {
		// Guard against unbounded memory growth: the whole plaintext bundle must
		// fit in memory because AES-GCM (pkg/crypt) is not a streaming mode.
		total += len(parsed.Hits.Hits)
		if total > exportMaxDocuments {
			return nil, errExportTooLarge
		}

		pageEntries, err := s.archiveScrollPage(ctx, zw, parsed.Hits.Hits)
		if err != nil {
			return nil, err
		}
		entries = append(entries, pageEntries...)

		next, err := s.exportScrollPage(ctx, scrollID)
		if err != nil {
			return nil, err
		}
		if next.ScrollID != "" {
			scrollID = next.ScrollID
		}
		parsed = next
	}

	return entries, nil
}

// retrievedPage holds the in-memory bytes of a successfully retrieved page along
// with its manifest entry. A nil entry (ok == false) marks a hit that was missing
// or failed to retrieve and must be skipped.
type retrievedPage struct {
	entry manifestEntry
	data  []byte
	ok    bool
}

// archiveScrollPage retrieves every hit in a single scroll page from storage
// CONCURRENTLY (bounded by exportRetrieveConcurrency), preserving scroll order,
// then writes the retrieved bytes into the zip SEQUENTIALLY. zip.Writer is not
// safe for concurrent use, so only the network-bound retrieval is parallelized;
// the archive is still assembled deterministically in scroll order. Pages that
// cannot be retrieved (model.ErrNotFound or any other retrieve error) are logged
// and skipped, matching the original best-effort behavior.
func (s *Server) archiveScrollPage(ctx context.Context, zw *zip.Writer, hits []exportHit) ([]manifestEntry, error) {
	// results is index-aligned with hits so the archive order is deterministic
	// regardless of which retrieval finishes first.
	results := make([]retrievedPage, len(hits))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(exportRetrieveConcurrency)

	for i := range hits {
		i := i
		hit := hits[i]
		g.Go(func() error {
			results[i] = s.retrievePage(gctx, hit)
			return nil
		})
	}
	// retrievePage never returns an error (missing pages are skipped), so Wait
	// only surfaces context cancellation.
	if err := g.Wait(); err != nil {
		return nil, err
	}

	entries := make([]manifestEntry, 0, len(hits))
	for _, res := range results {
		if !res.ok {
			continue
		}
		w, err := zw.Create(res.entry.FileName)
		if err != nil {
			log.Warnf("export: unable to create archive entry %s: %v", res.entry.FileName, err)
			continue
		}
		if _, err := w.Write(res.data); err != nil {
			log.Warnf("export: unable to write page %s into archive: %v", res.entry.FileName, err)
			continue
		}
		entries = append(entries, res.entry)
	}

	return entries, nil
}

// retrievePage fetches a single page's bytes from storage into memory. Missing
// pages (model.ErrNotFound) and other retrieve errors are logged and reported as
// a skip (ok == false) rather than failing the whole export.
func (s *Server) retrievePage(ctx context.Context, hit exportHit) retrievedPage {
	scanID := hit.Source.ScanID
	seq := hit.Source.SequenceID

	page, err := s.storage.Retrieve(ctx, scanID, seq)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			log.Warnf("export: page not found, skipping scan=%s seq=%d", scanID, seq)
			return retrievedPage{}
		}
		log.Warnf("export: unable to retrieve page scan=%s seq=%d: %v", scanID, seq, err)
		return retrievedPage{}
	}

	data, err := io.ReadAll(page.Reader)
	if err != nil {
		log.Warnf("export: unable to read page scan=%s seq=%d: %v", scanID, seq, err)
		return retrievedPage{}
	}

	fileName := fmt.Sprintf("files/%s_%d", scanID, seq)
	return retrievedPage{
		entry: manifestEntry{
			ScanID:        scanID,
			SequenceID:    seq,
			ContentDigest: hit.Source.ContentDigest,
			FileName:      fileName,
		},
		data: data,
		ok:   true,
	}
}

// exportScrollPage fetches the next page of a scroll, returning an empty
// response (no hits) once the scroll is exhausted.
func (s *Server) exportScrollPage(ctx context.Context, scrollID string) (exportScrollResponse, error) {
	var out exportScrollResponse
	if scrollID == "" {
		return out, nil
	}

	scrollResp, err := s.osClient.Scroll.Get(ctx, opensearchapi.ScrollGetReq{
		ScrollID: scrollID,
		Params: opensearchapi.ScrollGetParams{
			Scroll: exportScrollTTL,
		},
	})
	if err != nil {
		return out, err
	}
	defer scrollResp.Inspect().Response.Body.Close()
	if scrollResp.Inspect().Response.StatusCode >= 400 {
		return out, errExportSearchFailed
	}
	if err := json.NewDecoder(scrollResp.Inspect().Response.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

// writeArchiveFile writes a single named file into the zip writer.
func writeArchiveFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// signManifest computes a hex-encoded HMAC-SHA256 of the manifest bytes keyed by
// the server secret, mirroring the HMAC construction used for share tokens.
func signManifest(secret, manifest []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(manifest)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyManifest reports whether sig is a valid hex HMAC-SHA256 of manifest
// under secret. It is used by tests (and available to any verification tooling).
func verifyManifest(secret, manifest []byte, sig string) bool {
	want, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(manifest)
	return hmac.Equal(want, mac.Sum(nil))
}
