package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/handler"
	"api-gateway/internal/metrics"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"
	"api-gateway/internal/router"
	"api-gateway/internal/service/consul" // 导入 Consul 服务发现
	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel" // OpenTelemetry
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/zap"
)

var (
	cfgMutex   sync.RWMutex // 读写锁保护配置
	currentCfg *config.Config
)

func main() {
	// 初始加载配置
	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	updateConfig(cfg) // 设置全局配置

	// 初始化日志
	logger, err := setupLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()

	// 初始化 Jaeger 链路追踪 (如果启用)
	shutdownTracer, err := setupJaegerTracing(cfg.Jaeger, logger)
	if err != nil {
		logger.Warn("Jaeger 链路追踪初始化失败，继续运行但不启用追踪", zap.Error(err))
	}
	if shutdownTracer != nil {
		defer shutdownTracer(context.Background())
		logger.Info("Jaeger 链路追踪已启用")
	}

	reverseProxy := proxy.NewReverseProxy(logger)
	requestMetrics := metrics.NewRequestMetrics()

	// 初始化 Consul 服务发现客户端 (如果启用)
	var serviceDiscovery consul.ServiceDiscovery
	if cfg.ServiceDiscovery.Enabled && cfg.ServiceDiscovery.Type == "consul" {
		serviceDiscovery, err = consul.NewConsulServiceDiscovery(cfg.ServiceDiscovery.Consul.Address, logger)
		if err != nil {
			logger.Fatal("Consul 服务发现客户端初始化失败", zap.Error(err))
		}
		logger.Info("Consul 服务发现已启用", zap.String("address", cfg.ServiceDiscovery.Consul.Address))
	} else {
		logger.Info("服务发现未启用 (或配置为非 Consul 类型)")
	}

	// 初始化路由
	r := router.NewRouter()

	// 添加全局中间件
	r.Use(middleware.RecoverMiddleware(logger))
	r.Use(middleware.RequestLoggerMiddleware(logger))
	r.Use(metrics.MetricsMiddleware(requestMetrics))
	r.Use(middleware.RateLimiterMiddleware(func() config.RateLimitConfig { // 动态获取限流配置
		cfgMutex.RLock()
		defer cfgMutex.RUnlock()
		return currentCfg.RateLimit
	}(), logger))
	r.Use(middleware.AuthMiddleware(func() config.AuthConfig { // 动态获取认证配置
		cfgMutex.RLock()
		defer cfgMutex.RUnlock()
		return currentCfg.Auth
	}, logger))
	r.Use(middleware.OAuth2Middleware(func() config.AuthConfig { // 动态获取 OAuth 2.0 配置
		cfgMutex.RLock()
		defer cfgMutex.RUnlock()
		return currentCfg.Auth
	}, logger))
	r.Use(middleware.TracingMiddleware(shutdownTracer)) // 链路追踪中间件

	// 注册路由处理函数 (从配置加载路由规则)
	loadRoutes(r, reverseProxy, serviceDiscovery, logger)

	// 注册 metrics endpoint
	r.HandleFunc("/metrics", metrics.PrometheusHandler())

	// 启动 HTTP 服务器
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info("网关服务启动", zap.Int("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("服务启动失败", zap.Error(err))
		}
	}()

	// 启动配置动态加载 goroutine
	go watchConfigChanges("./config/config.yaml", logger, r, reverseProxy, serviceDiscovery)

	// 优雅停机信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGTERM)
	<-quit
	logger.Info("接收到停机信号，正在优雅关闭...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("网关服务关闭时发生错误", zap.Error(err))
	}

	logger.Info("网关服务已关闭")
}

// loadRoutes 从配置加载路由规则并注册处理函数
func loadRoutes(r *router.Router, reverseProxy *proxy.ReverseProxy, serviceDiscovery consul.ServiceDiscovery, logger *zap.Logger) {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	routes := currentCfg.Routes // 从全局配置获取路由规则

	r.ClearRoutes() // 清空现有路由规则，重新加载

	for _, route := range routes {
		var targetURL string
		if route.ServiceName != "" && serviceDiscovery != nil { // 使用服务发现
			serviceInstances, err := serviceDiscovery.GetServiceInstances(route.ServiceName)
			if err != nil {
				logger.Error("获取服务实例失败", zap.String("service_name", route.ServiceName), zap.Error(err))
				continue // 跳过当前路由
			}
			if len(serviceInstances) == 0 {
				logger.Warn("未找到服务实例", zap.String("service_name", route.ServiceName))
				continue // 跳过当前路由
			}
			//  这里简单选择第一个实例，实际场景中应实现负载均衡策略
			targetURL = fmt.Sprintf("http://%s:%d", serviceInstances[0].Host, serviceInstances[0].Port)
			logger.Debug("使用服务发现，路由到服务实例", zap.String("path", route.Path), zap.String("service_name", route.ServiceName), zap.String("target_url", targetURL))

		} else { // 使用静态 TargetURL (如果配置了)
			targetURL = route.TargetURL
			logger.Debug("使用静态 TargetURL", zap.String("path", route.Path), zap.String("target_url", targetURL))
		}

		if targetURL == "" {
			logger.Warn("路由目标 URL 未配置，跳过路由注册", zap.String("path", route.Path))
			continue // 跳过当前路由
		}

		timeout, err := time.ParseDuration(route.Timeout)
		if err != nil {
			logger.Warn("解析路由超时时间失败，使用默认超时时间", zap.String("path", route.Path), zap.Error(err))
			timeout = 10 * time.Second // 默认超时时间
		}
		getProxy, err := reverseProxy.GetProxy(targetURL)
		if err != nil {
			logger.Error("获取反向代理失败", zap.String("target_url", targetURL), zap.Error(err))
			continue // 跳过当前路由
		}
		r.HandleFunc(route.Path, handler.ProxyHandler(getProxy, targetURL, timeout, logger))
		logger.Info("注册路由", zap.String("path", route.Path), zap.String("target_url", targetURL), zap.Duration("timeout", timeout))
	}
	logger.Info("路由规则加载完成，共注册路由", zap.Int("route_count", len(routes)))
}

// watchConfigChanges 监听配置文件变化并热加载配置
func watchConfigChanges(configPath string, logger *zap.Logger, r *router.Router, reverseProxy *proxy.ReverseProxy, serviceDiscovery consul.ServiceDiscovery) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatal("创建文件监听器失败", zap.Error(err))
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
					logger.Info("配置文件发生变化，重新加载配置", zap.String("file", event.Name))
					if newCfg, err := config.LoadConfig(configPath); err == nil {
						updateConfig(newCfg)                                  // 更新全局配置
						loadRoutes(r, reverseProxy, serviceDiscovery, logger) // 重新加载路由
						logger.Info("配置重新加载完成")
					} else {
						logger.Error("重新加载配置失败", zap.Error(err))
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("文件监听器错误", zap.Error(err))
			case <-done:
				return
			}
		}
	}()

	err = watcher.Add(configPath)
	if err != nil {
		logger.Fatal("添加文件监听失败", zap.String("file", configPath), zap.Error(err))
	}
	<-done // 阻塞直到收到退出信号
}

// updateConfig 更新全局配置
func updateConfig(cfg *config.Config) {
	cfgMutex.Lock()
	defer cfgMutex.Unlock()
	currentCfg = cfg
}

// setupLogger 初始化 Zap 日志库 (与之前版本相同)
func setupLogger(logLevel string) (*zap.Logger, error) {
	level := zap.InfoLevel
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zap.DebugLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	}

	config := zap.NewProductionConfig()
	config.Level.SetLevel(level)
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("初始化日志配置失败: %w", err)
	}
	return logger, nil
}

// setupJaegerTracing 初始化 Jaeger 链路追踪
func setupJaegerTracing(jaegerConfig config.JaegerConfig, logger *zap.Logger) (shutdown func(ctx context.Context) error, err error) {
	if !jaegerConfig.Enabled {
		return nil, nil // 如果 Jaeger 未启用，则直接返回 nil
	}

	exporter, err := jaeger.New(jaeger.WithAgentEndpoint(jaeger.WithAgentHost(jaegerConfig.AgentAddress)))
	if err != nil {
		return nil, fmt.Errorf("创建 Jaeger Exporter 失败: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(jaegerConfig.ServiceName),
			semconv.ServiceVersion("1.0.0"), //  版本号可以从构建信息中获取
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Resource 失败: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()), //  全采样，生产环境可以调整采样率
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	shutdownFunc := func(ctx context.Context) error {
		//  优雅关闭 TracerProvider
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			logger.Error("TracerProvider 关闭失败", zap.Error(err))
			return err
		}
		return nil
	}
	return shutdownFunc, nil
}
