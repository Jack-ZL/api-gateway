package middleware

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

// RecoverMiddleware Panic 恢复中间件
func RecoverMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("服务 Panic 恢复",
						zap.Any("error", err),
						zap.String("stacktrace", string(debug.Stack())),
					)
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("服务器内部错误"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
