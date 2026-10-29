package hello

import (
	"context"

	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/model"
)

// SayHello 问候一次
func (s *HelloService) SayHello(ctx context.Context, req *hellosvc.SayHelloReq) (*hellosvc.SayHelloReply, error) {
	g, err := s.biz.SayHello(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return &hellosvc.SayHelloReply{Greeting: toProtoGreeting(g)}, nil
}

// ListGreeting 问候记录列表
func (s *HelloService) ListGreeting(ctx context.Context, req *hellosvc.ListGreetingReq) (*hellosvc.ListGreetingReply, error) {
	page := req.GetPage()
	list, total, err := s.biz.ListGreeting(ctx, model.ListGreetingParam{
		Name:     req.GetName(),
		Page:     int(page.GetCurrent()),
		PageSize: int(page.GetSize()),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*hellosvc.Greeting, 0, len(list))
	for _, g := range list {
		out = append(out, toProtoGreeting(g))
	}
	return &hellosvc.ListGreetingReply{Greetings: out, Total: total}, nil
}
