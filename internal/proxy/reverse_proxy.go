package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"go.uber.org/zap"
)

// ReverseProxy 封装反向代理
type ReverseProxy struct {
	proxies map[string]*httputil.ReverseProxy
	mu      sync.Mutex
	logger  *zap.Logger // 传入 logger
}

// NewReverseProxy 创建 ReverseProxy
func NewReverseProxy(logger *zap.Logger) *ReverseProxy {
	return &ReverseProxy{
		proxies: make(map[string]*httputil.ReverseProxy),
		logger:  logger, // 存储 logger
	}
}

// GetProxy 获取或创建指定 TargetURL 的反向代理
func (rp *ReverseProxy) GetProxy(targetURLStr string) (*httputil.ReverseProxy, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if p, ok := rp.proxies[targetURLStr]; ok {
		return p, nil
	}

	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		return nil, err
	}

	proxy := &httputil.ReverseProxy{ // 正确用法：直接使用结构体字面量创建 *httputil.ReverseProxy
		Director: func(req *http.Request) { // Director 函数用于修改转发请求
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path //  保留目标路径
			req.Host = targetURL.Host     //  需要显式设置 Host 头
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) { // ErrorHandler 自定义错误处理
			rp.logger.Error("反向代理错误", zap.String("path", r.URL.Path), zap.Error(err))
			w.WriteHeader(http.StatusBadGateway) // 返回 502 Bad Gateway 错误
			fmt.Fprintln(w, "反向代理错误")
		},
	}
	rp.proxies[targetURLStr] = proxy
	return proxy, nil
}
