// Package biz 业务逻辑层（内嵌 dao，数据库访问直接走 gen dao）
package biz

import (
	"context"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/internal/dao"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/storagex"

	// 注册供应商实现（minio / s3）
	_ "github.com/xiongpan828/beidou-layout/pkg/storagex/minio"
	_ "github.com/xiongpan828/beidou-layout/pkg/storagex/s3"
)

type biz struct {
	*dao.Dao
	cfg       *conf.Config
	providers sync.Map // code → cachedProvider
}

// NewBiz 创建业务聚合实例
func NewBiz(db *gorm.DB, cfg *conf.Config) model.IStorageBiz {
	return &biz{Dao: dao.NewDao(db), cfg: cfg}
}

// cachedProvider 供应商客户端 + 创建它时那行配置的 updated_at
type cachedProvider struct {
	updatedAt time.Time
	p         storagex.Provider
}

// provider 按 code 取供应商客户端。每次都读一行 provider（code 唯一索引，很便宜），
// updated_at 变了就换新客户端：后台改了配置，所有副本下一次请求就生效，不需要广播失效
func (b *biz) provider(ctx context.Context, code string) (storagex.Provider, error) {
	row, err := b.getProvider(ctx, code)
	if err != nil {
		return nil, err
	}
	if c, ok := b.providers.Load(code); ok && c.(cachedProvider).updatedAt.Equal(row.UpdatedAt) {
		return c.(cachedProvider).p, nil
	}
	p, err := storagex.New(row.Type, row.Config)
	if err != nil {
		return nil, err
	}
	b.providers.Store(code, cachedProvider{updatedAt: row.UpdatedAt, p: p})
	return p, nil
}

func (b *biz) getProvider(ctx context.Context, code string) (*model.Provider, error) {
	p, err := b.WithContext(ctx).Provider.Where(b.Q.Provider.Code.Eq(code)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, storagesvc.ErrorProviderNotFound("provider %q not found", code)
	}
	if err != nil {
		return nil, storagesvc.ErrorProviderGetFail("get provider %q: %v", code, err)
	}
	return p, nil
}
