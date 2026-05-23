package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/denysvitali/odi/pkg/reindex"
	"github.com/denysvitali/odi/pkg/storage/model"
)

const maxRecentReindexErrors = 25

var (
	errReindexIndexerUnavailable = errors.New("reindex endpoint not configured: OCR/Indexer not initialized")
	errReindexStorageUnsupported = errors.New("storage does not support page listing")
	errReindexAlreadyRunning     = errors.New("reindex is already running")
)

type reindexPageError struct {
	Page  string `json:"page"`
	Error string `json:"error"`
}

type reindexStatus struct {
	State       string             `json:"state"`
	StartedAt   *time.Time         `json:"startedAt,omitempty"`
	FinishedAt  *time.Time         `json:"finishedAt,omitempty"`
	Total       int                `json:"total"`
	Processed   int                `json:"processed"`
	Duplicates  int                `json:"duplicates"`
	Failed      int                `json:"failed"`
	CurrentPage string             `json:"currentPage,omitempty"`
	RecentError []reindexPageError `json:"recentErrors,omitempty"`
	Error       string             `json:"error,omitempty"`
}

func (s *Server) handleStartReindex(c *gin.Context) {
	status, err := s.StartReindex()
	if err != nil {
		switch {
		case errors.Is(err, errReindexAlreadyRunning):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "status": status})
		case errors.Is(err, errReindexIndexerUnavailable), errors.Is(err, errReindexStorageUnsupported):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "status": status})
		default:
			log.Errorf("unable to start reindex: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": status})
		}
		return
	}
	c.JSON(http.StatusAccepted, status)
}

func (s *Server) handleGetReindexStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.ReindexStatus())
}

func (s *Server) StartReindex() (reindexStatus, error) {
	if s.indexer == nil {
		return s.ReindexStatus(), errReindexIndexerUnavailable
	}
	lister, ok := s.storage.(model.PageLister)
	if !ok {
		return s.ReindexStatus(), errReindexStorageUnsupported
	}
	if !s.reindexProcessMu.TryLock() {
		return s.ReindexStatus(), errReindexAlreadyRunning
	}

	now := time.Now()
	s.setReindexStatus(reindexStatus{
		State:     "running",
		StartedAt: &now,
	})

	ctx := s.workerCtx
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		defer s.reindexProcessMu.Unlock()
		s.runReindex(ctx, lister)
	}()

	return s.ReindexStatus(), nil
}

func (s *Server) runReindex(ctx context.Context, lister model.PageLister) {
	pages, err := lister.ListPages(ctx)
	if err != nil {
		s.finishReindex("failed", fmt.Errorf("list pages: %w", err))
		return
	}
	s.updateReindexStatus(func(status *reindexStatus) {
		status.Total = len(pages)
	})

	result := reindex.Run(ctx, s.storage, s.indexer, pages, func(pageResult reindex.PageResult, result reindex.Result) {
		s.updateReindexStatus(func(status *reindexStatus) {
			status.Total = result.Total
			status.Processed = result.Processed
			status.Duplicates = result.Duplicates
			status.Failed = result.Failed
			status.CurrentPage = pageResult.Page.ID()
			if pageResult.Error != nil {
				status.RecentError = append(status.RecentError, reindexPageError{
					Page:  pageResult.Page.ID(),
					Error: pageResult.Error.Error(),
				})
				if len(status.RecentError) > maxRecentReindexErrors {
					status.RecentError = status.RecentError[len(status.RecentError)-maxRecentReindexErrors:]
				}
			}
		})
	})

	state := "completed"
	var runErr error
	if err := ctx.Err(); err != nil {
		state = "failed"
		runErr = err
	}
	s.updateReindexStatus(func(status *reindexStatus) {
		status.Total = result.Total
		status.Processed = result.Processed
		status.Duplicates = result.Duplicates
		status.Failed = result.Failed
	})
	s.finishReindex(state, runErr)
}

func (s *Server) finishReindex(state string, err error) {
	finishedAt := time.Now()
	s.updateReindexStatus(func(status *reindexStatus) {
		status.State = state
		status.FinishedAt = &finishedAt
		status.CurrentPage = ""
		if err != nil {
			status.Error = err.Error()
		}
	})
}

func (s *Server) ReindexStatus() reindexStatus {
	s.reindexStatusMu.Lock()
	defer s.reindexStatusMu.Unlock()

	status := s.reindexStatus
	if status.State == "" {
		status.State = "idle"
	}
	status.RecentError = append([]reindexPageError(nil), status.RecentError...)
	return status
}

func (s *Server) setReindexStatus(status reindexStatus) {
	s.reindexStatusMu.Lock()
	defer s.reindexStatusMu.Unlock()
	s.reindexStatus = status
}

func (s *Server) updateReindexStatus(update func(*reindexStatus)) {
	s.reindexStatusMu.Lock()
	defer s.reindexStatusMu.Unlock()
	update(&s.reindexStatus)
}
