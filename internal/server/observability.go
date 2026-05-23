package server

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// requestIDKey is the typed context key used to inject the per-request
// correlation ID into c.Request.Context().
type requestIDKey struct{}

// RequestIDFromContext returns the correlation ID stored on the request
// context, or the empty string if none is set.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

const requestIDHeader = "X-Request-ID"

// requestIDMiddleware ensures every request carries an X-Request-ID header.
// If the client did not supply one, a UUIDv4 is generated. The ID is stored
// on the request context under requestIDKey, set as a Gin key ("request_id"),
// and echoed back on the response header.
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDKey{}, id))
		c.Set("request_id", id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// Prometheus metrics — scoped to this server. We use a private registry so
// the global default registry is untouched; the handler is mounted on
// /metrics outside the auth group.
var (
	metricsRegistry = prometheus.NewRegistry()

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled, labelled by method, route template and status.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labelled by method and route template.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	metricsRegistry.MustRegister(httpRequestsTotal, httpRequestDuration)
	// Surface Go runtime + process collectors on /metrics as well.
	metricsRegistry.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
}

// metricsMiddleware records request counts and latency. The "path" label is
// the Gin route template (c.FullPath()) to avoid unbounded cardinality from
// dynamic URL segments. Requests that don't match a registered route are
// recorded under a sentinel value.
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// metricsHandler returns an http.Handler that serves the registered metrics.
func metricsHandler() gin.HandlerFunc {
	h := promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{Registry: metricsRegistry})
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
