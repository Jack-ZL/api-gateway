package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"api-gateway/internal/config"
	"go.uber.org/zap"
)

// OAuth2Middleware OAuth 2.0 客户端凭证模式认证中间件
func OAuth2Middleware(getAuthConfig func() config.AuthConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authConfig := getAuthConfig() // 动态获取认证配置

			if !authConfig.Enabled || strings.ToLower(authConfig.Type) != "oauth2" || !authConfig.OAuth2.Enabled {
				next.ServeHTTP(w, r) //  如果 OAuth 2.0 未启用，直接放行
				return
			}

			oauth2Config := authConfig.OAuth2
			tokenEndpoint := oauth2Config.TokenEndpoint
			clientID := oauth2Config.ClientID
			clientSecret := oauth2Config.ClientSecret

			//  这里为了简化，直接使用客户端凭证模式获取 token，实际生产环境可能需要更复杂的流程
			token, err := fetchOAuth2Token(tokenEndpoint, clientID, clientSecret, logger)
			if err != nil {
				logger.Warn("OAuth 2.0 认证：获取 Token 失败", zap.String("path", r.URL.Path), zap.Error(err))
				http.Error(w, "OAuth 2.0 认证失败", http.StatusUnauthorized)
				return
			}

			//  将 access_token 放入请求头，传递给后端服务 (实际情况可能需要根据后端服务的要求进行调整)
			r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
			next.ServeHTTP(w, r)
		})
	}
}

// OAuth2TokenResponse OAuth 2.0 Token 响应结构体
type OAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// fetchOAuth2Token 使用客户端凭证模式获取 OAuth 2.0 Token
func fetchOAuth2Token(tokenEndpoint, clientID, clientSecret string, logger *zap.Logger) (*OAuth2TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials") //  客户端凭证模式

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 OAuth 2.0 Token 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret) //  使用 Basic Auth 传递 client_id 和 client_secret

	client := &http.Client{Timeout: 10 * time.Second} //  设置超时时间
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth 2.0 Token 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth 2.0 Token 请求失败，状态码: %d", resp.StatusCode)
	}

	var tokenResp OAuth2TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("解析 OAuth 2.0 Token 响应失败: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("OAuth 2.0 Token 响应缺少 access_token")
	}

	logger.Debug("OAuth 2.0 成功获取 Token", zap.String("token_type", tokenResp.TokenType), zap.Int("expires_in", tokenResp.ExpiresIn))
	return &tokenResp, nil
}
