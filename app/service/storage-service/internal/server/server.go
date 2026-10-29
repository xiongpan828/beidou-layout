// Package server gRPC server（BFF 调用）+ 文件 HTTP server（网关 /api/v1/storage/* 转发）装配
package server

import (
	"time"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/internal/service/storage"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/pkg/constants"
	"github.com/xiongpan828/beidou-layout/pkg/validatex"
)

// NewGRPCServer 装配 gRPC server
func NewGRPCServer(c *conf.Config, svc *storage.StorageService) *grpc.Server {
	srv := grpc.NewServer(
		grpc.Address(c.GetGrpcAddr()),
		// kratos 默认 1s；签发地址会读库 + 首次创建供应商客户端
		grpc.Timeout(10*time.Second),
		grpc.Middleware(recovery.Recovery(), validatex.New()),
	)
	storagesvc.RegisterStorageServiceServer(srv, svc)
	return srv
}

// NewHTTPServer 文件读写 HTTP server：PUT 上传 / GET 下载，同一个路径，签名在 handler 里校验
func NewHTTPServer(c *conf.Config, svc *storage.StorageService) *http.Server {
	srv := http.NewServer(
		http.Address(c.GetHttpAddr()),
		// kratos 默认 1s 会掐断大文件；单个请求最长 10 分钟（大小由网关 client_max_body_size 限制）
		http.Timeout(10*time.Minute),
		http.Middleware(recovery.Recovery()),
	)
	r := srv.Route("/")
	r.PUT(constants.FilePathPrefix+"{key:.+}", svc.UploadFile)
	r.GET(constants.FilePathPrefix+"{key:.+}", svc.DownloadFile)
	return srv
}
