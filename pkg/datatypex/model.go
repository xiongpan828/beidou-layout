// Package datatypex 公共数据类型（gorm 基础模型等）
package datatypex

import (
	"time"

	"gorm.io/gorm"
)

// Model 基础模型（ID 自增 + 时间字段 + 软删除）。
// model 层与 migrations 层复用，字段稳定不轻易修改。
type Model struct {
	ID        uint32 `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
