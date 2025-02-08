package middleware

import (
	"net/http"

	"api-gateway/internal/config"
	"api-gateway/pkg/ratelimiter"
	"go.uber.org/zap"
)

// RateLimiterMiddleware 限流中间件
func RateLimiterMiddleware(rateLimitConfig config.RateLimitConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	if !rateLimitConfig.Enabled {
		return func(next http.Handler) http.Handler {
			return next // 如果未启用限流，则直接放行
		}
	}

	limiter := ratelimiter.NewTokenBucketLimiter(
		rateLimitConfig.Requests,
		rateLimitConfig.Interval,
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				logger.Warn("请求被限流", zap.String("path", r.URL.Path))
				http.Error(w, "请求过于频繁，请稍后重试", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
