package middleware

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "api-gateway-middleware" //  Tracer 名称

// TracingMiddleware 链路追踪中间件
func TracingMiddleware(shutdownTracer func(ctx context.Context) error) func(http.Handler) http.Handler {
	if shutdownTracer == nil { //  如果 Jaeger 未启用，直接返回 no-op 中间件
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	tracer := otel.Tracer(tracerName) // 获取 Tracer

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx, span := tracer.Start(ctx, "gateway-request-handling", trace.WithSpanKind(trace.SpanKindServer)) //  创建 Span
			defer span.End()                                                                                     // 确保 Span 结束

			r = r.WithContext(ctx) // 将带有 Span 的 Context 传递下去
			next.ServeHTTP(w, r)
		})
	}
}
