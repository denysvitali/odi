package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type readyCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type readyResponse struct {
	Ready          bool         `json:"ready"`
	IngestionReady bool         `json:"ingestionReady"`
	Checks         []readyCheck `json:"checks"`
}

// handleReadyz checks OpenSearch and, when ingestion is configured, OCR
// and configured Zefix dependencies. Read-only servers do not require an
// indexer. Returns 503 if a required dependency is unhealthy.
func (s *Server) handleReadyz(c *gin.Context) {
	resp := s.readinessReport(c.Request.Context())
	status := http.StatusOK
	if !resp.Ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
}

func (s *Server) readinessReport(ctx context.Context) readyResponse {
	var checks []readyCheck

	// OpenSearch is always wired on the server, regardless of indexer presence.
	osCheck := readyCheck{Name: "opensearch", OK: true}
	if err := s.pingOs(ctx); err != nil {
		osCheck.OK = false
		osCheck.Detail = "dependency unavailable"
	}
	checks = append(checks, osCheck)

	// A missing indexer is an intentional read-only deployment. Ingestion
	// dependencies matter only when ingestion was configured.

	if s.indexer != nil {
		ocrCheck := readyCheck{Name: "ocr", OK: true}
		ok, err := s.indexer.PingOcrApiContext(ctx)
		if err != nil {
			ocrCheck.OK = false
			ocrCheck.Detail = "dependency unavailable"
		} else if !ok {
			ocrCheck.OK = false
			ocrCheck.Detail = "OCR API is not healthy"
		}
		checks = append(checks, ocrCheck)

		// Zefix is optional — only check if it was configured
		if s.indexer.IsZefixConfigured() {
			zefixCheck := readyCheck{Name: "zefix", OK: true}
			if err := s.indexer.PingZefixContext(ctx); err != nil {
				zefixCheck.OK = false
				zefixCheck.Detail = "dependency unavailable"
			}
			checks = append(checks, zefixCheck)
		}
	}

	ready := true
	for _, ch := range checks {
		if !ch.OK {
			ready = false
			break
		}
	}
	return readyResponse{Ready: ready, IngestionReady: ready && s.indexer != nil, Checks: checks}
}
