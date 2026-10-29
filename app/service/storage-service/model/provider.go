package model

import (
	"context"

	"github.com/xiongpan828/beidou-layout/pkg/datatypex"
)

// Provider 对象存储供应商（配置 JSON 存库，结构见 storage_service.MinioConfig / S3Config）。
// 删除是硬删（code 可以重新创建）
type Provider struct {
	datatypex.Model
	Code   string `gorm:"type:varchar(100);not null;uniqueIndex"`
	Name   string `gorm:"type:varchar(100);not null"`
	Type   string `gorm:"type:varchar(100);not null"`
	Config string `gorm:"type:jsonb;not null;default:'{}'"`
}

// CreateProviderParam 创建供应商参数
type CreateProviderParam struct {
	Code   string
	Name   string
	Type   string
	Config string
}

// UpdateProviderParam 更新供应商参数（按 code 定位；Config 里密钥留空 = 不改）
type UpdateProviderParam struct {
	Code   string
	Name   string
	Config string
}

// IProviderBiz 供应商业务接口
type IProviderBiz interface {
	ListProvider(ctx context.Context) ([]*Provider, error)
	CreateProvider(ctx context.Context, param CreateProviderParam) (*Provider, error)
	UpdateProvider(ctx context.Context, param UpdateProviderParam) (*Provider, error)
	DeleteProvider(ctx context.Context, code string) error
}
