package model

import (
	"context"

	"github.com/xiongpan828/beidou-layout/pkg/datatypex"
)

// Greeting 问候记录
type Greeting struct {
	datatypex.Model
	Name    string `gorm:"type:varchar(50);not null;index"`
	Message string `gorm:"type:varchar(200);not null"`
}

// ListGreetingParam 问候记录列表参数
type ListGreetingParam struct {
	Name     string
	Page     int // 从 1 开始；0 = 第一页
	PageSize int // 0 = 默认 20（上限由 proto common.Page 校验）
}

// IHelloBiz hello-service 业务接口（service 与 biz 之间的契约）
type IHelloBiz interface {
	// SayHello 问候一次并记录
	SayHello(ctx context.Context, name string) (*Greeting, error)
	// ListGreeting 问候记录列表
	ListGreeting(ctx context.Context, param ListGreetingParam) ([]*Greeting, int64, error)
}
