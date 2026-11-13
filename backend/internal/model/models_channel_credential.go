package model

import "time"

type ChannelCredentialSource string

const (
	ChannelCredentialSourcePlatform ChannelCredentialSource = "platform"
	ChannelCredentialSourceUser     ChannelCredentialSource = "user"
)

type UserChannelCredential struct {
	UserID       string    `json:"-" gorm:"primaryKey;size:36"`
	ChannelID    string    `json:"-" gorm:"primaryKey;size:36;index"`
	APIKeyCipher string    `json:"-" gorm:"type:text;not null"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}
