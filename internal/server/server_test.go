package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/odi/pkg/models"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logrus.StandardLogger().SetLevel(logrus.DebugLevel)
	os.Exit(m.Run())
}

type mockThumbnailStorage struct {
	exists           bool
	existsErr        error
	thumb            *models.ThumbnailPage
	retrieveThumbErr error
	storeThumbErr    error
	storeThumbCalled bool
}

func (m *mockThumbnailStorage) ThumbnailExists(ctx context.Context, scanID string, sequenceNumber int) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockThumbnailStorage) RetrieveThumbnail(ctx context.Context, scanID string, sequenceNumber int) (*models.ThumbnailPage, error) {
	return m.thumb, m.retrieveThumbErr
}

func (m *mockThumbnailStorage) StoreThumbnail(ctx context.Context, scanID string, sequenceNumber int, reader io.Reader) error {
	m.storeThumbCalled = true
	return m.storeThumbErr
}

type mockRWStorage struct {
	pages       map[string]*models.ScannedPage
	retrieveErr error
}

func newMockRWStorage() *mockRWStorage {
	return &mockRWStorage{
		pages: make(map[string]*models.ScannedPage),
	}
}

func (m *mockRWStorage) Store(ctx context.Context, page models.ScannedPage) error {
	m.pages[page.ID()] = &page
	return nil
}

func (m *mockRWStorage) Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error) {
	if m.retrieveErr != nil {
		return nil, m.retrieveErr
	}
	page, ok := m.pages[pageKey(scanID, sequenceNumber)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return page, nil
}

func pageKey(scanID string, sequenceNumber int) string {
	return fmt.Sprintf("%s_%d", scanID, sequenceNumber)
}

func (m *mockRWStorage) addPage(scanID string, sequenceID int, data []byte) {
	reader := bytes.NewReader(data)
	m.pages[pageKey(scanID, sequenceID)] = &models.ScannedPage{
		Reader:     reader,
		ScanID:     scanID,
		SequenceID: sequenceID,
	}
}

type mockServer struct {
	*Server
	storage         *mockRWStorage
	thumbStorage    *mockThumbnailStorage
	router          *gin.Engine
}

func setupTestServer(thumbStorage *mockThumbnailStorage, rwStorage *mockRWStorage) *mockServer {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	ms := &mockServer{
		storage:      rwStorage,
		thumbStorage: thumbStorage,
		router:       router,
	}

	storage := &mockCombinedStorage{
		mockRWStorage:  rwStorage,
		mockThumbnailStorage: thumbStorage,
	}

	s := &Server{
		storage: storage,
		e:       router,
	}

	ms.Server = s

	router.GET("/api/v1/thumbnails/:id", s.handleGetThumbnail)

	return ms
}

type mockCombinedStorage struct {
	*mockRWStorage
	*mockThumbnailStorage
}

func (m *mockCombinedStorage) Store(ctx context.Context, page models.ScannedPage) error {
	return m.mockRWStorage.Store(ctx, page)
}

func (m *mockCombinedStorage) Retrieve(ctx context.Context, scanID string, sequenceNumber int) (*models.ScannedPage, error) {
	return m.mockRWStorage.Retrieve(ctx, scanID, sequenceNumber)
}

func (m *mockCombinedStorage) ThumbnailExists(ctx context.Context, scanID string, sequenceNumber int) (bool, error) {
	return m.mockThumbnailStorage.ThumbnailExists(ctx, scanID, sequenceNumber)
}

func (m *mockCombinedStorage) RetrieveThumbnail(ctx context.Context, scanID string, sequenceNumber int) (*models.ThumbnailPage, error) {
	return m.mockThumbnailStorage.RetrieveThumbnail(ctx, scanID, sequenceNumber)
}

func (m *mockCombinedStorage) StoreThumbnail(ctx context.Context, scanID string, sequenceNumber int, reader io.Reader) error {
	return m.mockThumbnailStorage.StoreThumbnail(ctx, scanID, sequenceNumber, reader)
}

func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestHandleGetThumbnail_InvalidID(t *testing.T) {
	thumbStorage := &mockThumbnailStorage{}
	rwStorage := newMockRWStorage()
	ms := setupTestServer(thumbStorage, rwStorage)

	tests := []struct {
		name string
		id   string
		code int
	}{
		{"no underscore", "abc123", http.StatusBadRequest},
		{"invalid scan id", "not-a-valid-uuid_1", http.StatusBadRequest},
		{"invalid sequence id", "89aefd17-4e1c-4339-bbc7-3bd0ca40a34c_abc", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/thumbnails/"+tt.id, nil)
			w := httptest.NewRecorder()
			ms.router.ServeHTTP(w, req)

			if w.Code != tt.code {
				t.Errorf("expected status %d, got %d", tt.code, w.Code)
			}
		})
	}
}

func TestHandleGetThumbnail_NotFound(t *testing.T) {
	thumbStorage := &mockThumbnailStorage{exists: false}
	rwStorage := newMockRWStorage()
	ms := setupTestServer(thumbStorage, rwStorage)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/thumbnails/89aefd17-4e1c-4339-bbc7-3bd0ca40a34c_1", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHandleGetThumbnail_RetrieveError(t *testing.T) {
	thumbStorage := &mockThumbnailStorage{exists: false}
	rwStorage := newMockRWStorage()
	rwStorage.retrieveErr = errors.New("some error")
	ms := setupTestServer(thumbStorage, rwStorage)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/thumbnails/89aefd17-4e1c-4339-bbc7-3bd0ca40a34c_1", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestHandleGetThumbnail_ExistingThumbnail(t *testing.T) {
	thumbStorage := &mockThumbnailStorage{
		exists: true,
		thumb: &models.ThumbnailPage{
			Reader:     bytes.NewReader(createTestPNG()),
			ScanID:     "89aefd17-4e1c-4339-bbc7-3bd0ca40a34c",
			SequenceID: 1,
		},
	}
	rwStorage := newMockRWStorage()
	ms := setupTestServer(thumbStorage, rwStorage)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/thumbnails/89aefd17-4e1c-4339-bbc7-3bd0ca40a34c_1", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", w.Header().Get("Content-Type"))
	}
}

func TestHandleGetThumbnail_GenerateNewThumbnail(t *testing.T) {
	thumbStorage := &mockThumbnailStorage{exists: false}
	rwStorage := newMockRWStorage()
	rwStorage.addPage("89aefd17-4e1c-4339-bbc7-3bd0ca40a34c", 1, createTestPNG())
	ms := setupTestServer(thumbStorage, rwStorage)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/thumbnails/89aefd17-4e1c-4339-bbc7-3bd0ca40a34c_1", nil)
	w := httptest.NewRecorder()
	ms.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", w.Header().Get("Content-Type"))
	}
	if !thumbStorage.storeThumbCalled {
		t.Error("expected StoreThumbnail to be called")
	}
}