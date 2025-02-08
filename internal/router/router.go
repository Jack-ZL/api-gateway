package router

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Router 封装 gorilla/mux.Router
type Router struct {
	*mux.Router
	middlewares []func(http.Handler) http.Handler
}

// NewRouter 创建一个新的 Router
func NewRouter() *Router {
	return &Router{
		Router: mux.NewRouter(),
	}
}

// Use 添加中间件
func (r *Router) Use(middleware func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, middleware)
}

// HandleFunc 注册路由处理函数，并应用中间件
func (r *Router) HandleFunc(path string, handler http.HandlerFunc) {
	// 倒序应用中间件，保证中间件执行顺序
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler).(http.HandlerFunc)
	}
	r.Router.HandleFunc(path, handler) // 直接使用传参的 handler
}

// ClearRoutes 清空所有已注册的路由规则
func (r *Router) ClearRoutes() {
	r.Router = mux.NewRouter() //  直接创建一个新的 Router 实例即可清空
}
