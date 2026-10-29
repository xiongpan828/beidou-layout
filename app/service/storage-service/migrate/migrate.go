// Package migrate 迁移入口
package migrate

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/xiongpan828/beidou-layout/app/service/storage-service/migrate/migrations"
)

// Migrate 执行数据库迁移
func Migrate(db *gorm.DB) error {
	// 库里有不认识的迁移 ID（例如删掉 / 合并过迁移却没清库）直接报错，避免旧表结构被悄悄沿用
	opts := *gormigrate.DefaultOptions
	opts.ValidateUnknownMigrations = true
	m := gormigrate.New(db, &opts, migrations.Migrations)
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
