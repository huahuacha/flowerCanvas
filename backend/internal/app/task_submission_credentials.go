package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
)

type taskSubmissionCredential struct {
	UserID        string           `json:"userId"`
	TaskID        string           `json:"taskId"`
	AttemptID     string           `json:"attemptId"`
	ChannelID     string           `json:"channelId"`
	BaseURL       string           `json:"baseUrl"`
	ProxyURL      string           `json:"proxyUrl"`
	APIFormat     string           `json:"apiFormat"`
	InterfaceType string           `json:"interfaceType"`
	Model         string           `json:"model"`
	APIKey        string           `json:"apiKey"`
	Headers       []OutboundHeader `json:"headers"`
}

type providerRouteAttemptContextKey struct{}
type fixedProviderKeyContextKey struct{}

type fixedProviderRequestCredential struct {
	key     string
	baseURL string
}

func (s *Service) saveTaskSubmissionCredential(ctx context.Context, userID string, config providerConfig) error {
	if config.credentialSource != model.ChannelCredentialSourceUser {
		return nil
	}
	attempt, _ := ctx.Value(providerRouteAttemptContextKey{}).(*model.RouteAttempt)
	if attempt == nil || attempt.TaskID != taskExecutionID(ctx) || (attempt.ChannelID != "" && attempt.ChannelID != config.ChannelID) {
		return errors.New("无法确认固定渠道的任务提交记录")
	}
	payload, err := json.Marshal(taskSubmissionCredential{
		UserID: userID, TaskID: attempt.TaskID, AttemptID: attempt.ID, ChannelID: config.ChannelID,
		BaseURL: config.BaseURL, ProxyURL: config.proxyURL, APIFormat: config.APIFormat, InterfaceType: config.InterfaceType,
		Model: config.Model, APIKey: config.APIKey, Headers: config.Headers,
	})
	if err != nil {
		return err
	}
	cipher, err := s.encryptSettingSecret(string(payload))
	if err != nil {
		return err
	}
	if err := s.repo.WithContext(ctx).SaveRouteAttemptSubmissionCredential(attempt.ID, config.ChannelID, cipher); err != nil {
		return err
	}
	attempt.ChannelID = config.ChannelID
	return nil
}

func (s *Service) applyTaskSubmissionCredential(task model.Task, attempt *model.RouteAttempt, config providerConfig) (providerConfig, error) {
	if !strings.HasPrefix(attempt.SubmissionCredentialCipher, encryptedSettingPrefix) {
		return providerConfig{}, errors.New("固定渠道任务凭据格式无效")
	}
	plaintext, err := s.decryptSettingSecret(attempt.SubmissionCredentialCipher)
	if err != nil {
		return providerConfig{}, err
	}
	var credential taskSubmissionCredential
	if json.Unmarshal([]byte(plaintext), &credential) != nil || credential.TaskID != task.ID || credential.UserID != task.UserID ||
		credential.AttemptID != attempt.ID || credential.ChannelID == "" || credential.ChannelID != attempt.ChannelID ||
		strings.TrimSpace(credential.APIKey) == "" || credential.InterfaceType == "" || credential.Model == "" {
		return providerConfig{}, errors.New("固定渠道任务凭据与原提交记录不一致")
	}
	if _, err := ValidateOutboundURL(credential.BaseURL); err != nil {
		return providerConfig{}, err
	}
	config.ChannelID = credential.ChannelID
	config.BaseURL, config.proxyURL = credential.BaseURL, credential.ProxyURL
	config.APIFormat, config.InterfaceType = credential.APIFormat, credential.InterfaceType
	config.Model, config.APIKey, config.SecretKey, config.Headers = credential.Model, credential.APIKey, "", credential.Headers
	config.credentialSource = model.ChannelCredentialSourceUser
	return config, nil
}

func (s *Service) resolveSubmittedTaskProviderConfig(task *model.Task, config providerConfig) (providerConfig, error) {
	if task.ID == "" || (task.LogicalModelID == "" && firstNonEmpty(config.ChannelID, systemChannelIDFromBaseURL(config.BaseURL)) == "") {
		return s.resolveProviderConfig(config)
	}
	// A manual retry creates a new route run. Old credentials never follow it.
	attempt, err := s.repo.LatestRouteAttempt(task.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return providerConfig{}, err
	}
	if attempt != nil && attempt.RouteRun == task.RouteRun && attempt.SubmissionCredentialCipher != "" {
		return s.applyTaskSubmissionCredential(*task, attempt, config)
	}
	return s.resolveProviderConfig(config)
}

func (s *Service) resumeFixedChannelTask(ctx context.Context, input canvasGenerationInput) (map[string]interface{}, error) {
	ctx = withFixedProviderCredential(ctx, input.Config)
	ctx = ensureOfficialProtocolAdapter(ctx, input.Config.InterfaceType)
	if adapter, ok := declarativeProtocolAdapterForContext(ctx, input.Config.InterfaceType); ok {
		return runProtocolAdapterTask(ctx, input, adapter)
	}
	if input.Mode == "video" && s.videoProviderSupportsTaskResume(input.Config.InterfaceType) {
		return runVideoTask(ctx, input)
	}
	if input.Mode == "audio" && input.Config.InterfaceType == string(model.ChannelInterfaceAsyncAudio) {
		return runAudioTask(ctx, input)
	}
	return nil, errors.New("当前协议不支持查询原任务")
}

func withFixedProviderCredential(ctx context.Context, config providerConfig) context.Context {
	if config.credentialSource != model.ChannelCredentialSourceUser {
		return ctx
	}
	if analytics, ok := ctx.Value(providerAnalyticsKey{}).(providerAnalyticsContext); ok {
		analytics.ChannelID = config.ChannelID
		analytics.ProxyURL = config.proxyURL
		ctx = context.WithValue(ctx, providerAnalyticsKey{}, analytics)
	}
	return context.WithValue(ctx, fixedProviderKeyContextKey{}, fixedProviderRequestCredential{key: config.APIKey, baseURL: config.BaseURL})
}

func (credential fixedProviderRequestCredential) authorize(req *http.Request, client *http.Client) error {
	if credential.key == "" || !credential.sentBy(req) {
		return nil
	}
	origin, err := url.Parse(credential.baseURL)
	if err != nil || req.URL.Scheme != origin.Scheme || !strings.EqualFold(req.URL.Host, origin.Host) {
		return errors.New("固定渠道凭据只能用于该渠道配置的上游地址")
	}
	checkRedirect := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, previous []*http.Request) error {
		if next.URL.Scheme != origin.Scheme || !strings.EqualFold(next.URL.Host, origin.Host) {
			return errors.New("固定渠道鉴权请求不能重定向到其他上游地址")
		}
		if checkRedirect != nil {
			return checkRedirect(next, previous)
		}
		return nil
	}
	return nil
}

func (credential fixedProviderRequestCredential) sentBy(req *http.Request) bool {
	if user, password, ok := req.BasicAuth(); ok && (user == credential.key || password == credential.key) {
		return true
	}
	for _, values := range req.Header {
		for _, value := range values {
			if strings.Contains(value, credential.key) {
				return true
			}
		}
	}
	return strings.Contains(req.URL.RawQuery, url.QueryEscape(credential.key))
}

type providerNotSubmittedError struct{ error }

func (e providerNotSubmittedError) Unwrap() error { return e.error }

func redactFixedProviderCredential(payload []byte, credential string) []byte {
	return bytes.ReplaceAll(payload, []byte(credential), []byte("[REDACTED]"))
}

func redactFixedTextValue(value any, key string) any {
	switch v := value.(type) {
	case string:
		return strings.ReplaceAll(v, key, "[REDACTED]")
	case map[string]any:
		for field, child := range v {
			v[field] = redactFixedTextValue(child, key)
		}
	case []any:
		for index, child := range v {
			v[index] = redactFixedTextValue(child, key)
		}
	}
	return value
}

func redactFixedTextDeltas(publish func(string), key string) (func(string), func()) {
	redactor := fixedProviderStreamRedactor{credential: []byte(key)}
	emit := func(chunk []byte, final bool) {
		if text := redactor.consume(chunk, final); len(text) > 0 {
			publish(string(text))
		}
	}
	return func(delta string) { emit([]byte(delta), false) }, func() { emit(nil, true) }
}

type fixedProviderStreamRedactor struct {
	credential []byte
	pending    []byte
}

func (r *fixedProviderStreamRedactor) consume(chunk []byte, final bool) []byte {
	data := append(r.pending, chunk...)
	var output []byte
	for {
		position := bytes.Index(data, r.credential)
		if position < 0 {
			break
		}
		output = append(output, data[:position]...)
		output = append(output, "[REDACTED]"...)
		data = data[position+len(r.credential):]
	}
	keep := 0
	if !final {
		// A key echoed by upstream can span arbitrary transport chunk boundaries.
		for size := min(len(data), len(r.credential)-1); size > 0; size-- {
			if bytes.HasSuffix(data, r.credential[:size]) {
				keep = size
				break
			}
		}
	}
	output = append(output, data[:len(data)-keep]...)
	r.pending = append(r.pending[:0], data[len(data)-keep:]...)
	return output
}
