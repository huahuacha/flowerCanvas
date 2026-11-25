package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
)

func (s *Service) checkFixedChannelForAdjustment(ctx context.Context, userID, channelID string) error {
	if channelID == "" {
		return fixedChannelMissingKey()
	}
	channel, err := s.repo.SystemChannel(channelID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && channel.CredentialSource != model.ChannelCredentialSourceUser) {
		return NotFound("固定渠道不存在或已停用")
	}
	if err != nil {
		return err
	}
	key, err := s.fixedChannelAPIKey(userID, channel)
	if err != nil {
		return err
	}
	headers, err := ParseOutboundHeadersJSON(channel.HeadersJSON)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL(channel.BaseURL, "/v1/models"), nil)
	if err != nil {
		return BadAuthRequest("固定渠道的模型列表地址无效，请联系管理员")
	}
	req.Header.Set("Accept", "application/json")
	ApplyOutboundHeaders(req, headers)
	ApplyDefaultOutboundHeaders(req)
	req.Header.Set("Authorization", "Bearer "+key)
	if _, err := ValidateOutboundURLWithProxy(req.URL.String(), channel.ProxyURL); err != nil {
		return BadAuthRequest("固定渠道的模型列表地址不可访问，请联系管理员")
	}
	client := OutboundHTTPClientWithProxy(8*time.Second, channel.ProxyURL)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	credential := fixedProviderRequestCredential{key: key, baseURL: channel.BaseURL}
	if err := credential.authorize(req, client); err != nil {
		return BadAuthRequest("固定渠道的模型列表地址不可访问，请联系管理员")
	}
	resp, err := client.Do(req)
	if err != nil {
		return &AppError{Status: http.StatusBadGateway, Code: http.StatusBadGateway, Message: "暂时无法读取固定渠道模型列表，请稍后再试或联系管理员", Retryable: true}
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return BadAuthRequest("固定渠道拒绝了当前 Key，请检查 Key 或模型权限")
	case http.StatusTooManyRequests:
		return &AppError{Status: http.StatusTooManyRequests, Code: http.StatusTooManyRequests, Message: "固定渠道暂时限流，请稍后再试", Retryable: true}
	default:
		return &AppError{Status: http.StatusBadGateway, Code: http.StatusBadGateway, Message: "暂时无法读取固定渠道模型列表，请稍后再试或联系管理员", Retryable: true}
	}
	const maxModelListBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelListBytes+1))
	if err != nil || len(body) > maxModelListBytes {
		return BadAuthRequest("固定渠道的模型列表无法读取，请联系管理员")
	}
	var result struct {
		Success *bool `json:"success"`
		Data    []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || (result.Success != nil && !*result.Success) {
		return BadAuthRequest("固定渠道未返回可用模型列表，请检查 Key 或联系管理员")
	}
	for _, item := range result.Data {
		if strings.TrimSpace(item.ID) != "" {
			return nil
		}
	}
	return BadAuthRequest("当前 Key 没有可用模型，请检查固定渠道的模型权限")
}
