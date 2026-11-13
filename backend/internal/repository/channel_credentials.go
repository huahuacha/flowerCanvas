package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yingce/backend/internal/model"
)

func (r *Repository) UserChannelCredential(userID, channelID string) (*model.UserChannelCredential, error) {
	var credential model.UserChannelCredential
	if err := r.db.First(&credential, "user_id = ? AND channel_id = ?", userID, channelID).Error; err != nil {
		return nil, err
	}
	return &credential, nil
}

func (r *Repository) UserChannelCredentialStatuses(userID string) ([]model.UserChannelCredential, error) {
	var credentials []model.UserChannelCredential
	err := r.db.Select("channel_id", "updated_at").Where("user_id = ?", userID).Find(&credentials).Error
	return credentials, err
}

func (r *Repository) SaveUserChannelCredential(credential *model.UserChannelCredential) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var channel model.ModelChannel
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&channel,
			"id = ? AND scope = ? AND enabled = ? AND credential_source = ?", credential.ChannelID, model.ChannelScopeSystem, true, model.ChannelCredentialSourceUser).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "channel_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"api_key_cipher", "updated_at"}),
		}).Create(credential).Error
	})
}

func (r *Repository) DeleteUserChannelCredential(userID, channelID string) error {
	return r.db.Where("user_id = ? AND channel_id = ?", userID, channelID).Delete(&model.UserChannelCredential{}).Error
}
