package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// RequestLoggerMiddleware 请求日志中间件 (使用 Zap)
func RequestLoggerMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)
			duration := time.Since(startTime)

			logger.Info("请求处理完成",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status_code", ww.statusCode),
				zap.Duration("duration", duration),
			)
		})
	}
}

// responseWriterWrapper 用于包装 http.ResponseWriter 并记录状态码
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
