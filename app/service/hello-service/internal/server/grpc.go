// Package server gRPC server 装配
package server

import (
	"time"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/hello_service"
	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/internal/service/hello"
	"github.com/xiongpan828/beidou-layout/pkg/validatex"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer 装配 gRPC server
func NewGRPCServer(c *conf.Config, helloSvc *hello.HelloService) *grpc.Server {
	srv := grpc.NewServer(
		grpc.Address(c.GetGrpcAddr()),
		// kratos 默认 1s，DB 慢查询时容易误超时
		grpc.Timeout(10*time.Second),
		grpc.Middleware(recovery.Recovery(), validatex.New()),
	)
	hellosvc.RegisterHelloServiceServer(srv, helloSvc)
	return srv
}
