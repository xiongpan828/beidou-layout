package main

import (
	"context"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/api_service"
	"github.com/xiongpan828/beidou-layout/app/service/api-service/initializer"
	"github.com/xiongpan828/beidou-layout/app/service/api-service/internal/biz"
	"github.com/xiongpan828/beidou-layout/app/service/api-service/internal/server"
	hellosvcapp "github.com/xiongpan828/beidou-layout/app/service/api-service/internal/service/hello"
	"github.com/xiongpan828/beidou-layout/app/service/api-service/pkg/constants"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
	"github.com/xiongpan828/beidou-layout/pkg/loggingx"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// newServer 装配 api-service 依赖链：config → initializer.Clients → biz → service → http server。
// 返回 http server 与资源清理函数。
func newServer() (*khttp.Server, func(), error) {
	var bc conf.Config
	if err := configx.Load(flagconf, &bc); err != nil {
		return nil, nil, err
	}
	loggingx.Init(constants.ServiceName)

	// 下游 client/代理
	clients, cleanup, err := initializer.NewClients(context.Background(), &bc)
	if err != nil {
		return nil, nil, err
	}

	// biz（聚合下游）→ service → server
	helloSvc := hellosvcapp.NewHello(biz.NewHelloBiz(clients.Hello))
	srv := server.NewHTTPServer(&bc, helloSvc)
	return srv, cleanup, nil
}
