package database

import (
	"fmt"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
)

var fixedChannelsMigration = migration{
	version: 111, name: "personal_fixed_channels", checksum: "sha256:personal-fixed-channels-v111", apply: migrateFixedChannels,
}

func migrateFixedChannels(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&model.UserChannelCredential{}); err != nil {
		return err
	}
	for _, column := range []struct {
		table any
		field string
	}{
		{&model.ModelChannel{}, "CredentialSource"},
		{&model.RouteAttempt{}, "SubmissionCredentialCipher"},
	} {
		if tx.Migrator().HasTable(column.table) && !tx.Migrator().HasColumn(column.table, column.field) {
			if err := tx.Migrator().AddColumn(column.table, column.field); err != nil {
				return fmt.Errorf("固定渠道迁移新增 %s: %w", column.field, err)
			}
		}
	}
	return nil
}
