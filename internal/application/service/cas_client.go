package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// CASClient CAS 接口客户端
type CASClient struct {
	config     *config.CASConfig
	httpClient *http.Client
}

// NewCASClient 创建 CAS 客户端
func NewCASClient(cfg *config.Config) *CASClient {
	var casConfig *config.CASConfig
	if cfg != nil {
		casConfig = cfg.CAS
	}
	return &CASClient{
		config: casConfig,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CASAPIResponse CAS API 响应结构
type CASAPIResponse struct {
	Code      int                `json:"code"`
	Data      *types.CASUserInfo `json:"data"`
	Msg       string             `json:"msg"`
	Exception string             `json:"exception"`
}

// ValidateSession 验证 CAS 会话（通过 _cas_sid 和 _cas_uid）
// referer 参数用于设置 Referer 头，CAS API 需要此头进行校验
func (c *CASClient) ValidateSession(ctx context.Context, casSid, casUid string, referer string) (*types.CASUserInfo, error) {
	if c.config == nil {
		return nil, fmt.Errorf("CAS config is not initialized")
	}

	// 获取当前环境配置
	envConfig := c.config.GetCurrentConfig()
	if envConfig == nil {
		return nil, fmt.Errorf("CAS environment config is not available")
	}

	// 构建 API URL（主接口 3.0）
	apiURL := fmt.Sprintf("https://%s/api/nxin.usercenter.user.get/3.0", envConfig.APIHost)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 Cookie（模拟浏览器行为）
	req.AddCookie(&http.Cookie{
		Name:  envConfig.CookieSID,
		Value: casSid,
	})
	req.AddCookie(&http.Cookie{
		Name:  envConfig.CookieUID,
		Value: casUid,
	})

	// 设置 User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 设置 Referer 头（CAS API 需要此头进行校验）
	if referer != "" {
		req.Header.Set("Referer", referer)
	} else {
		// 如果没有提供 Referer，使用默认值（根据环境配置）
		defaultReferer := fmt.Sprintf("https://%s/", envConfig.LoginHost)
		req.Header.Set("Referer", defaultReferer)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warnf(ctx, "CAS API request failed: %v, trying fallback API", err)
		// 如果主接口失败，尝试备用接口（2.0版本）
		return c.validateSessionFallback(ctx, casSid, casUid, envConfig, referer)
	}
	defer resp.Body.Close()

	// 解析响应
	var result CASAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 检查响应码
	if result.Code != 0 {
		// 如果主接口返回未登录错误，尝试备用接口
		if result.Code == 10011 {
			logger.Debugf(ctx, "CAS API returned login required (code: %d), trying fallback API", result.Code)
			return c.validateSessionFallback(ctx, casSid, casUid, envConfig, referer)
		}
		return nil, fmt.Errorf("CAS API error: code=%d, msg=%s, exception=%s", result.Code, result.Msg, result.Exception)
	}

	if result.Data == nil {
		return nil, fmt.Errorf("CAS API returned empty data")
	}

	return result.Data, nil
}

// validateSessionFallback 使用备用接口验证会话（2.0版本，需要 boId 参数）
func (c *CASClient) validateSessionFallback(ctx context.Context, casSid, casUid string, envConfig *config.CASEnvConfig, referer string) (*types.CASUserInfo, error) {
	// 备用接口需要先通过主接口获取用户ID，如果主接口失败，备用接口也无法使用
	// 这里先尝试主接口获取用户ID，如果失败则返回错误
	// 注意：备用接口主要用于主接口返回部分数据时使用，如果主接口完全失败，备用接口也无法工作

	// 先尝试主接口获取用户ID（即使返回错误，也可能有部分数据）
	apiURL := fmt.Sprintf("https://%s/api/nxin.usercenter.user.get/3.0", envConfig.APIHost)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback request: %w", err)
	}

	req.AddCookie(&http.Cookie{
		Name:  envConfig.CookieSID,
		Value: casSid,
	})
	req.AddCookie(&http.Cookie{
		Name:  envConfig.CookieUID,
		Value: casUid,
	})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 设置 Referer 头
	if referer != "" {
		req.Header.Set("Referer", referer)
	} else {
		defaultReferer := fmt.Sprintf("https://%s/", envConfig.LoginHost)
		req.Header.Set("Referer", defaultReferer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallback API request failed: %w", err)
	}
	defer resp.Body.Close()

	var result CASAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode fallback response: %w", err)
	}

	// 如果主接口返回了用户数据（即使code不为0），使用主接口数据
	if result.Data != nil && result.Data.ID != "" {
		return result.Data, nil
	}

	// 如果主接口完全失败，备用接口也无法使用（需要boId参数）
	return nil, fmt.Errorf("CAS session validation failed: code=%d, msg=%s", result.Code, result.Msg)
}
