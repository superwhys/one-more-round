package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var operations = promauto.NewCounterVec(prometheus.CounterOpts{Name: "omr_operations_total", Help: "Completed diary operations by operation and result."}, []string{"operation", "result"})
var latency = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "omr_operation_duration_seconds", Help: "Diary operation latency.", Buckets: prometheus.DefBuckets}, []string{"operation"})

// operationOf names the observed product operations; other requests are not
// counted. Labels never carry an account, a memory or a photo.
func operationOf(c *gin.Context) string {
	path := c.FullPath()
	switch {
	case strings.HasSuffix(path, "/photos") && c.Request.Method == http.MethodPost:
		return "photo_upload"
	case strings.Contains(path, "/rounds") && (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut):
		return "round_save"
	case strings.HasSuffix(path, "/bgg/search"):
		return "external_search"
	default:
		return ""
	}
}

// observeOperations records the success and the latency of the observed
// product operations.
func observeOperations() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		operation := operationOf(c)
		if operation == "" {
			return
		}
		result := "success"
		if c.Writer.Status() >= 400 {
			result = "failure"
		}
		operations.WithLabelValues(operation, result).Inc()
		latency.WithLabelValues(operation).Observe(time.Since(start).Seconds())
	}
}
