package metrics

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RequestMetrics 请求指标
type RequestMetrics struct {
	requestTotal    *prometheus.CounterVec
	errorTotal      *prometheus.CounterVec
	requestLatency  *prometheus.HistogramVec
	lastRequestTime atomic.Int64
}

// NewRequestMetrics 创建 RequestMetrics
func NewRequestMetrics() *RequestMetrics {
	requestTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_gateway_requests_total",
		Help: "Total requests received by the gateway.",
	}, []string{"path", "method"})

	errorTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_gateway_errors_total",
		Help: "Total errors encountered by the gateway.",
	}, []string{"path", "method", "status_code"})

	requestLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_gateway_request_latency_seconds",
		Help:    "Request latency in seconds.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10}, // 常用 buckets
	}, []string{"path", "method"})

	prometheus.MustRegister(requestTotal, errorTotal, requestLatency)

	return &RequestMetrics{
		requestTotal:    requestTotal,
		errorTotal:      errorTotal,
		requestLatency:  requestLatency,
		lastRequestTime: atomic.Int64{},
	}
}

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware(reqMetrics *RequestMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			reqMetrics.requestTotal.WithLabelValues(r.URL.Path, r.Method).Inc()

			ww := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)

			duration := time.Since(startTime)
			reqMetrics.requestLatency.WithLabelValues(r.URL.Path, r.Method).Observe(duration.Seconds())

			if ww.statusCode >= 400 {
				reqMetrics.errorTotal.WithLabelValues(r.URL.Path, r.Method, strconv.Itoa(ww.statusCode)).Inc()
			}

			reqMetrics.lastRequestTime.Store(time.Now().UnixNano())
		})
	}
}

// statusResponseWriter 用于包装 http.ResponseWriter 并记录状态码
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// PrometheusHandler Prometheus Metrics Handler
func PrometheusHandler() http.HandlerFunc {
	return promhttp.Handler().(http.HandlerFunc)
}
