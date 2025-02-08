package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"api-gateway/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// AuthMiddleware 认证中间件 (支持 JWT 和 OAuth 2.0)
func AuthMiddleware(getAuthConfig func() config.AuthConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authConfig := getAuthConfig() // 动态获取认证配置

			if !authConfig.Enabled {
				next.ServeHTTP(w, r) //  如果认证未启用，直接放行
				return
			}

			authType := strings.ToLower(authConfig.Type)

			switch authType {
			case "jwt":
				jwtAuth(authConfig.JWT, next, logger).ServeHTTP(w, r) //  JWT 认证
			case "oauth2":
				// OAuth 2.0 认证 (这里可以调用单独的 OAuth 2.0 中间件，或者直接在此处实现 OAuth 2.0 客户端凭证模式的验证)
				//  为了代码简洁，这里先留空，OAuth 2.0 验证逻辑放到 OAuth2Middleware 中实现
				next.ServeHTTP(w, r) //  OAuth 2.0 验证交给 OAuth2Middleware 处理
			case "none":
				next.ServeHTTP(w, r) //  不进行认证
			default:
				logger.Warn("未知的认证类型，跳过认证", zap.String("auth_type", authType))
				next.ServeHTTP(w, r) //  未知认证类型，默认放行
			}
		})
	}
}

// jwtAuth JWT 认证处理 (与之前版本相同，无需修改)
func jwtAuth(jwtConfig config.JWTAuthConfig, next http.Handler, logger *zap.Logger) http.Handler {
	secretKey := []byte(jwtConfig.SecretKey)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Warn("JWT 认证：未提供 Authorization Header", zap.String("path", r.URL.Path))
			http.Error(w, "未授权", http.StatusUnauthorized)
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("无效的签名方法: %v", token.Header["alg"])
			}
			return secretKey, nil
		})

		if err != nil {
			logger.Warn("JWT 认证：Token 解析失败", zap.String("path", r.URL.Path), zap.Error(err))
			http.Error(w, "无效的Token", http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			ctx := context.WithValue(r.Context(), "claims", claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			logger.Warn("JWT 认证：Token 验证失败", zap.String("path", r.URL.Path))
			http.Error(w, "无效的Token", http.StatusUnauthorized)
		}
	})
}
