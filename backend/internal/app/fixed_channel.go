package app

import (
	"errors"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
	"yingce/backend/internal/kernel"
	"yingce/backend/internal/model"
)

const fixedChannelMissingKeyHint = "请先在个人设置 → 渠道与存储 → 固定渠道填写该渠道的 API Key"

type FixedChannelView struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Enabled             bool       `json:"enabled"`
	HasCredential       bool       `json:"hasCredential"`
	CredentialUpdatedAt *time.Time `json:"credentialUpdatedAt,omitempty"`
}

type FixedChannelList struct {
	Channels []FixedChannelView `json:"channels"`
}

func fixedChannelMissingKey() *kernel.AppError {
	err := BadAuthRequest(fixedChannelMissingKeyHint)
	err.Reason = "FIXED_CHANNEL_CREDENTIAL_REQUIRED"
	return err
}

func fixedChannelTeamUnavailable() *kernel.AppError {
	return Forbidden("固定渠道仅可在个人工作区使用")
}

func (s *Service) requireActiveTaskOwner(userID string) (*model.User, error) {
	user, err := s.repo.User(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, Forbidden("账号不存在或已删除")
	}
	if err != nil {
		return nil, err
	}
	if user.Status != model.UserStatusActive {
		return nil, Forbidden("账号已停用，不能提交新的上游请求")
	}
	return user, nil
}

func (s *Service) requireChannelEligibility(access *userGroupAccess, channel model.ModelChannel) error {
	if err := access.requireChannel(channel.ID); err != nil {
		return err
	}
	if channel.CredentialSource == model.ChannelCredentialSourceUser {
		if access.teamWorkspace {
			return fixedChannelTeamUnavailable()
		}
		if !access.group.AllChannels && !access.group.channels[channel.ID] {
			return ModelCatalogMismatch("当前账号分组无权使用该模型，请刷新后重新选择")
		}
		if access.userID == "" {
			return Unauthorized("请先登录")
		}
	}
	return nil
}

func taskSelectedSystemChannelID(input map[string]any) string {
	config, _ := input["config"].(map[string]any)
	return firstNonEmpty(stringValue(config["channelId"]), systemChannelIDFromBaseURL(stringValue(config["baseUrl"])))
}

func (s *Service) requireFixedTaskCredential(userID string, input map[string]any) error {
	channelID := taskSelectedSystemChannelID(input)
	if channelID == "" {
		return nil
	}
	channel, err := s.repo.SystemChannel(channelID)
	if err != nil {
		return InvalidModelSelection("指定的渠道不存在或已停用")
	}
	if channel.CredentialSource != model.ChannelCredentialSourceUser {
		return nil
	}
	_, err = s.fixedChannelAPIKey(userID, channel)
	return err
}

func (s *Service) fixedChannelCredential(userID, channelID string) (*model.UserChannelCredential, error) {
	row, err := s.repo.UserChannelCredential(userID, channelID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && row.APIKeyCipher == "") {
		return nil, fixedChannelMissingKey()
	}
	return row, err
}

func (s *Service) fixedChannelAPIKey(userID string, channel *model.ModelChannel) (string, error) {
	if _, team := model.TeamIDFromContentKey(userID); team {
		return "", fixedChannelTeamUnavailable()
	}
	user, err := s.requireActiveTaskOwner(userID)
	if err != nil {
		return "", err
	}
	access, err := s.userGroupAccessForUser(user)
	if err != nil {
		return "", err
	}
	if err := s.requireChannelEligibility(access, *channel); err != nil {
		return "", err
	}
	credential, err := s.fixedChannelCredential(userID, channel.ID)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(credential.APIKeyCipher, encryptedSettingPrefix) {
		return "", BadAuthRequest("固定渠道凭据无效，请重新配置 API Key")
	}
	return s.decryptSettingSecret(credential.APIKeyCipher)
}

func (s *Service) FixedChannels(user *model.User) (*FixedChannelList, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	activeUser, err := s.requireActiveTaskOwner(user.ID)
	if err != nil {
		return nil, err
	}
	access, err := s.userGroupAccessForUser(activeUser)
	if err != nil {
		return nil, err
	}
	channels, err := s.repo.SystemChannels(false)
	if err != nil {
		return nil, err
	}
	credentials, err := s.repo.UserChannelCredentialStatuses(user.ID)
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]time.Time, len(credentials))
	for _, credential := range credentials {
		statuses[credential.ChannelID] = credential.UpdatedAt
	}
	result := &FixedChannelList{Channels: make([]FixedChannelView, 0)}
	for _, channel := range channels {
		if channel.CredentialSource == model.ChannelCredentialSourceUser && (access.group.AllChannels || access.group.channels[channel.ID]) {
			view := FixedChannelView{ID: channel.ID, Name: channel.Name, Enabled: channel.Enabled}
			if updatedAt, ok := statuses[channel.ID]; ok {
				view.HasCredential = true
				view.CredentialUpdatedAt = &updatedAt
			}
			result.Channels = append(result.Channels, view)
		}
	}
	return result, nil
}

func (s *Service) SaveFixedChannelCredential(user *model.User, channelID, apiKey string) (*FixedChannelView, error) {
	if user == nil {
		return nil, Unauthorized("请先登录")
	}
	activeUser, err := s.requireActiveTaskOwner(user.ID)
	if err != nil {
		return nil, err
	}
	access, err := s.userGroupAccessForUser(activeUser)
	if err != nil {
		return nil, err
	}
	channel, err := s.repo.SystemChannel(channelID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && channel.CredentialSource != model.ChannelCredentialSourceUser) {
		return nil, NotFound("固定渠道不存在或已停用")
	}
	if err != nil {
		return nil, err
	}
	if err := s.requireChannelEligibility(access, *channel); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(apiKey)
	if key == "" || len(key) > 4096 || strings.ContainsFunc(key, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return nil, BadAuthRequest("请填写有效的固定渠道 API Key")
	}
	cipher, err := s.encryptSettingSecret(key)
	if err != nil {
		return nil, err
	}
	credential := model.UserChannelCredential{UserID: user.ID, ChannelID: channel.ID, APIKeyCipher: cipher}
	if err := s.repo.SaveUserChannelCredential(&credential); err != nil {
		return nil, err
	}
	return &FixedChannelView{ID: channel.ID, Name: channel.Name, Enabled: channel.Enabled, HasCredential: true, CredentialUpdatedAt: &credential.UpdatedAt}, nil
}

func (s *Service) DeleteFixedChannelCredential(user *model.User, channelID string) error {
	if user == nil {
		return Unauthorized("请先登录")
	}
	// Revocation stays available after a channel is disabled or group access is removed.
	return s.repo.DeleteUserChannelCredential(user.ID, channelID)
}
