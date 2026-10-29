package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/xiongpan828/beidou-layout/pkg/datatypex"
)

// 问候记录。
// 迁移里定义自己的结构体快照（不引用 model），之后 model 改字段不会改变历史迁移的行为。
func init() {
	type Greeting struct {
		datatypex.Model
		Name    string `gorm:"type:varchar(50);not null;index"`
		Message string `gorm:"type:varchar(200);not null"`
	}

	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20261029000000",
		Migrate: func(db *gorm.DB) error {
			return db.AutoMigrate(&Greeting{})
		},
		Rollback: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&Greeting{})
		},
	})
}
