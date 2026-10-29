// Package biz 业务逻辑层（内嵌 dao，数据库访问直接走 gen dao）
package biz

import (
	"gorm.io/gorm"

	"github.com/xiongpan828/beidou-layout/app/service/hello-service/internal/dao"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/model"
)

type biz struct {
	*dao.Dao
}

// NewBiz 创建业务聚合实例
func NewBiz(db *gorm.DB) model.IHelloBiz {
	return &biz{Dao: dao.NewDao(db)}
}
