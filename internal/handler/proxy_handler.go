package handler

import (
	"context"
	"net/http"
	"net/http/httputil"
	"time"

	"go.uber.org/zap"
)

// ProxyHandler 创建反向代理处理函数
func ProxyHandler(proxy *httputil.ReverseProxy, targetURL string, timeout time.Duration, logger *zap.Logger) http.HandlerFunc {
	director := func(req *http.Request) {
		req.URL.Scheme = "http" // 或 "https"
		req.URL.Host = targetURL
		req.Host = targetURL

		// 请求头转换示例：添加自定义请求头
		req.Header.Set("X-Gateway-Request", "true")
		// 可以根据需要删除或修改其他请求头
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout) // 设置请求超时
		defer cancel()

		proxy.Director = director
		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) { // 自定义错误处理
			logger.Error("反向代理请求失败",
				zap.String("path", req.URL.Path),
				zap.String("target_url", targetURL),
				zap.Error(err),
			)
			rw.WriteHeader(http.StatusBadGateway) // 返回 502 Bad Gateway
			_, _ = rw.Write([]byte("后端服务不可用"))
		}

		// 使用 context.WithTimeout 创建带有超时控制的请求
		proxy.ServeHTTP(w, r.WithContext(ctx))
	}
}
