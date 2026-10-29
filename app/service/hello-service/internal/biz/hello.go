package biz

import (
	"context"
	"fmt"

	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/model"
)

// SayHello 问候一次并记录
func (b *biz) SayHello(ctx context.Context, name string) (*model.Greeting, error) {
	g := &model.Greeting{Name: name, Message: fmt.Sprintf("Hello, %s!", name)}
	if err := b.WithContext(ctx).Greeting.Create(g); err != nil {
		return nil, hellosvc.ErrorGreetingCreateFail("create greeting failed: %+v", err)
	}
	return g, nil
}

// ListGreeting 问候记录列表（新的在前）
func (b *biz) ListGreeting(ctx context.Context, param model.ListGreetingParam) ([]*model.Greeting, int64, error) {
	// 没传 page 时的默认值（传了的已由 proto common.Page 校验范围）
	page, size := max(param.Page, 1), param.PageSize
	if size == 0 {
		size = 20
	}
	q := b.Q.Greeting
	query := b.WithContext(ctx).Greeting
	if param.Name != "" {
		query = query.Where(q.Name.Eq(param.Name))
	}
	list, total, err := query.Order(q.ID.Desc()).FindByPage((page-1)*size, size)
	if err != nil {
		return nil, 0, hellosvc.ErrorGreetingGetFail("list greeting failed: %+v", err)
	}
	return list, total, nil
}
