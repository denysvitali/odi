// Package server implements the Gin HTTP API for the ODI document indexer.
package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// authMiddleware enforces a static bearer token on all incoming requests.
// It is registered on the /api/v1 group only; public endpoints
// (/healthz, /readyz, /metrics) bypass it because they are not part of the
// group.
func authMiddleware(token string) gin.HandlerFunc {
	expected := []byte(token)
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		provided := []byte(strings.TrimPrefix(header, prefix))
		if subtle.ConstantTimeCompare(provided, expected) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
