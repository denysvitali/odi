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
	Ready  bool         `json:"ready"`
	Checks []readyCheck `json:"checks"`
}

// handleReadyz reports whether the server is ready to accept ingestion
// requests: indexer configured, OpenSearch reachable, OCR API healthy,
// Zefix database reachable. Returns 503 if any dependency is unhealthy.
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
		osCheck.Detail = err.Error()
	}
	checks = append(checks, osCheck)

	// Indexer is optional on the server (e.g. read-only deployments), but
	// ingestion requires it.
	indexerCheck := readyCheck{Name: "indexer", OK: s.indexer != nil}
	if !indexerCheck.OK {
		indexerCheck.Detail = "indexer not configured (OCR_API_ADDR missing on the server) — upload endpoint is disabled"
	}
	checks = append(checks, indexerCheck)

	if s.indexer != nil {
		ocrCheck := readyCheck{Name: "ocr", OK: true}
		ok, err := s.indexer.PingOcrApi()
		if err != nil {
			ocrCheck.OK = false
			ocrCheck.Detail = err.Error()
		} else if !ok {
			ocrCheck.OK = false
			ocrCheck.Detail = "OCR API is not healthy"
		}
		checks = append(checks, ocrCheck)

		// Zefix is optional — only check if it was configured
		if s.indexer.IsZefixConfigured() {
			zefixCheck := readyCheck{Name: "zefix", OK: true}
			if err := s.indexer.PingZefix(); err != nil {
				zefixCheck.OK = false
				zefixCheck.Detail = err.Error()
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
	return readyResponse{Ready: ready, Checks: checks}
}
