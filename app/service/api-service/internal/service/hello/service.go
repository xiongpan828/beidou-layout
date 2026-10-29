// Package hello 实现 C 端问候 HTTP 接口。
// service 只做 pb ↔ model 转换，下游调用与聚合在 biz。
package hello

import (
	"context"

	apihello "github.com/xiongpan828/beidou-layout/api/gen/go/service/api_service/api_hello"
	"github.com/xiongpan828/beidou-layout/app/service/api-service/model"
)

// Hello 实现 service.api_service.api_hello.Hello 接口
type Hello struct {
	apihello.UnimplementedHelloServer
	biz model.IHelloBiz
}

// NewHello 创建 C 端问候服务
func NewHello(biz model.IHelloBiz) *Hello {
	return &Hello{biz: biz}
}

// SayHello 问候
func (s *Hello) SayHello(ctx context.Context, req *apihello.SayHelloReq) (*apihello.SayHelloReply, error) {
	msg, err := s.biz.SayHello(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return &apihello.SayHelloReply{Message: msg}, nil
}
