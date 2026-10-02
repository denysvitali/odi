package server

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestProductionRoutesRequireAuthentication(t *testing.T) {
	s := &Server{e: gin.New(), apiToken: "synthetic-token"}
	s.initRoutes()
	routes := map[string]bool{}
	for _, route := range s.e.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, path := range []string{"/reminders", "/insights", "/insights.csv", "/documents/example/similar"} {
		template := path
		if strings.Contains(path, "/similar") {
			template = "/documents/:id/similar"
		}
		require.True(t, routes["GET /api/v1"+template])
		w := httptest.NewRecorder()
		s.e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	for _, route := range s.e.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		parts := strings.Split(route.Path, "/")
		for i, part := range parts {
			if strings.HasPrefix(part, ":") {
				parts[i] = "synthetic"
			}
		}
		w := httptest.NewRecorder()
		s.e.ServeHTTP(w, httptest.NewRequest(route.Method, strings.Join(parts, "/"), nil))
		require.Equal(t, http.StatusUnauthorized, w.Code, route.Method+" "+route.Path)
	}
}

func TestAccessLogsOmitIdentifiersAndQueries(t *testing.T) {
	logger := logrus.StandardLogger()
	output := logger.Out
	defer logger.SetOutput(output)
	var logs bytes.Buffer
	logger.SetOutput(&logs)
	s := &Server{e: gin.New(), apiToken: "synthetic-token"}
	s.initRoutes()
	w := httptest.NewRecorder()
	s.e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/documents/private-document?secret=private-query", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, logs.String(), "/api/v1/documents/:id")
	require.NotContains(t, logs.String(), "private-document")
	require.NotContains(t, logs.String(), "private-query")
	w = httptest.NewRecorder()
	s.e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/share/private-share-token?p=private-passphrase", nil))
	require.Contains(t, logs.String(), "/share/:token")
	require.NotContains(t, logs.String(), "private-share-token")
	require.NotContains(t, logs.String(), "private-passphrase")
	require.Equal(t, "unmatched", requestRoute(httptest.NewRequest(http.MethodGet, "/unknown", nil)))
}

func TestUploadSequenceIDs(t *testing.T) {
	for _, tc := range []struct {
		name, ids, offset string
		count             int
		want              []int
		invalid           bool
	}{
		{name: "out of order", ids: "[8,2,5]", count: 3, want: []int{8, 2, 5}},
		{name: "legacy", offset: "3", count: 2, want: []int{4, 5}},
		{name: "mismatch", ids: "[1]", count: 2, invalid: true},
		{name: "duplicate", ids: "[2,2]", count: 2, invalid: true},
		{name: "nonpositive", ids: "[0]", count: 1, invalid: true},
		{name: "overflow", offset: fmt.Sprint(math.MaxInt), count: 1, invalid: true},
		{name: "malformed", ids: "invalid", count: 1, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := parseUploadSequenceIDs(tc.ids, tc.offset, tc.count)
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, ids)
			}
		})
	}
}
