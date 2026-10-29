// Package server HTTP server 装配
package server

import (
	"time"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/admin_service"
	adminhello "github.com/xiongpan828/beidou-layout/api/gen/go/service/admin_service/admin_hello"
	adminstorage "github.com/xiongpan828/beidou-layout/api/gen/go/service/admin_service/admin_storage"
	hellosvcapp "github.com/xiongpan828/beidou-layout/app/service/admin-service/internal/service/hello"
	storagesvcapp "github.com/xiongpan828/beidou-layout/app/service/admin-service/internal/service/storage"
	"github.com/xiongpan828/beidou-layout/pkg/codecx"
	"github.com/xiongpan828/beidou-layout/pkg/proxyx"
	"github.com/xiongpan828/beidou-layout/pkg/validatex"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/http"
)

// NewHTTPServer 装配 HTTP server（admin-service 只暴露 HTTP，南北流量）
func NewHTTPServer(c *conf.Config, helloSvc *hellosvcapp.Hello, storageSvc *storagesvcapp.Storage) *http.Server {
	opts := []http.ServerOption{
		http.Address(c.GetHttpAddr()),
		// 整个请求的上限（kratos 默认 1s 太短）：一个请求可能串行调几个下游（各自默认 2s）
		http.Timeout(15 * time.Second),
		// 注意：http.Middleware 多次调用会覆盖（matcher.Use 是赋值语义），中间件必须一次性传入。
		// 顺序：参数校验 → 代理转发（未实现的方法按配置转给下游）
		http.Middleware(
			recovery.Recovery(),
			validatex.New(),
			proxyx.Server(),
		),
	}
	// proto 消息用 protojson 编解码（snake_case 字段名 + RFC3339 Timestamp）
	opts = append(opts, codecx.ProtoJSON()...)

	srv := http.NewServer(opts...)
	adminhello.RegisterHelloHTTPServer(srv, helloSvc)
	adminstorage.RegisterStorageHTTPServer(srv, storageSvc)
	return srv
}
