package biz

import (
	"context"
	"slices"
	"strings"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/storagex"
)

// ListProvider 供应商列表
func (b *biz) ListProvider(ctx context.Context) ([]*model.Provider, error) {
	list, err := b.WithContext(ctx).Provider.Order(b.Q.Provider.ID).Find()
	if err != nil {
		return nil, storagesvc.ErrorProviderGetFail("list provider: %v", err)
	}
	return list, nil
}

// CreateProvider 创建供应商（配置按类型解码 + buf validate）
func (b *biz) CreateProvider(ctx context.Context, param model.CreateProviderParam) (*model.Provider, error) {
	config, err := storagex.Normalize(param.Type, param.Config, "")
	if err != nil {
		return nil, err
	}
	n, err := b.WithContext(ctx).Provider.Where(b.Q.Provider.Code.Eq(param.Code)).Count()
	if err != nil {
		return nil, storagesvc.ErrorProviderGetFail("check provider %q: %v", param.Code, err)
	}
	if n > 0 {
		return nil, storagesvc.ErrorProviderCodeExists("provider %q already exists", param.Code)
	}
	p := &model.Provider{Code: param.Code, Name: param.Name, Type: param.Type, Config: config}
	if err := b.WithContext(ctx).Provider.Create(p); err != nil {
		return nil, storagesvc.ErrorProviderCreateFail("create provider %q: %v", param.Code, err)
	}
	return p, nil
}

// UpdateProvider 更新名称与配置（密钥留空沿用旧值）；updated_at 变化后各副本自动换新客户端
func (b *biz) UpdateProvider(ctx context.Context, param model.UpdateProviderParam) (*model.Provider, error) {
	old, err := b.getProvider(ctx, param.Code)
	if err != nil {
		return nil, err
	}
	config, err := storagex.Normalize(old.Type, param.Config, old.Config)
	if err != nil {
		return nil, err
	}
	q := b.Q.Provider
	if _, err := b.WithContext(ctx).Provider.Where(q.ID.Eq(old.ID)).UpdateSimple(q.Name.Value(param.Name), q.Config.Value(config)); err != nil {
		return nil, storagesvc.ErrorProviderUpdateFail("update provider %q: %v", param.Code, err)
	}
	return b.getProvider(ctx, param.Code)
}

// DeleteProvider 删除供应商：被 scope 配置或已有文件引用时拒绝（否则这些文件就读不出来了）
func (b *biz) DeleteProvider(ctx context.Context, code string) error {
	p, err := b.getProvider(ctx, code)
	if err != nil {
		return err
	}
	var scopes []string
	for name, sc := range b.cfg.GetScopes() {
		if sc.GetProviderCode() == code {
			scopes = append(scopes, name)
		}
	}
	if len(scopes) > 0 {
		slices.Sort(scopes)
		return storagesvc.ErrorProviderInUse("provider %q is used by scope(s) %s", code, strings.Join(scopes, ","))
	}
	n, err := b.WithContext(ctx).File.Where(b.Q.File.ProviderCode.Eq(code)).Count()
	if err != nil {
		return storagesvc.ErrorFileGetFail("count files of provider %q: %v", code, err)
	}
	if n > 0 {
		return storagesvc.ErrorProviderInUse("provider %q still has %d file(s)", code, n)
	}
	if _, err := b.WithContext(ctx).Provider.Unscoped().Where(b.Q.Provider.ID.Eq(p.ID)).Delete(); err != nil {
		return storagesvc.ErrorProviderDeleteFail("delete provider %q: %v", code, err)
	}
	return nil
}
