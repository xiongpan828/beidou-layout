// Package biz api-service BFF 业务层：聚合下游 gRPC 服务
package biz

import (
	"context"

	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/api-service/model"
)

// helloBiz C 端问候业务实现：转发 hello-service
type helloBiz struct {
	client hellosvc.HelloServiceClient
}

// NewHelloBiz 创建问候业务实例，返回 model.IHelloBiz 接口
func NewHelloBiz(client hellosvc.HelloServiceClient) model.IHelloBiz {
	return &helloBiz{client: client}
}

// SayHello 问候一次，返回问候语
func (b *helloBiz) SayHello(ctx context.Context, name string) (string, error) {
	reply, err := b.client.SayHello(ctx, &hellosvc.SayHelloReq{Name: name})
	if err != nil {
		return "", err
	}
	return reply.GetGreeting().GetMessage(), nil
}
