package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/xiongpan828/beidou-layout/pkg/datatypex"
)

// 文件（签发上传地址时登记，写入成功后改状态）与对象存储供应商（配置 JSON 存 jsonb）。
// 迁移里定义自己的结构体快照（不引用 model）
func init() {
	type File struct {
		datatypex.Model
		Key          string `gorm:"type:varchar(1024);not null;uniqueIndex"`
		Filename     string `gorm:"type:varchar(255);not null;default:''"`
		Size         int64  `gorm:"not null;default:0"`
		ContentType  string `gorm:"type:varchar(100);not null;default:''"`
		Etag         string `gorm:"type:varchar(100);not null;default:''"`
		Status       string `gorm:"type:varchar(100);not null;default:FILE_STATUS_PENDING"`
		Scope        string `gorm:"type:varchar(100);not null"`
		ProviderCode string `gorm:"type:varchar(100);not null;index"`
	}
	type Provider struct {
		datatypex.Model
		Code   string `gorm:"type:varchar(100);not null;uniqueIndex"`
		Name   string `gorm:"type:varchar(100);not null"`
		Type   string `gorm:"type:varchar(100);not null"`
		Config string `gorm:"type:jsonb;not null;default:'{}'"`
	}

	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20261029100000",
		Migrate: func(db *gorm.DB) error {
			return db.AutoMigrate(&File{}, &Provider{})
		},
		Rollback: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&File{}, &Provider{})
		},
	})
}
