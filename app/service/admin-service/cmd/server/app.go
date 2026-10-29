package main

import (
	"context"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/admin_service"
	"github.com/xiongpan828/beidou-layout/app/service/admin-service/initializer"
	"github.com/xiongpan828/beidou-layout/app/service/admin-service/internal/server"
	hellosvcapp "github.com/xiongpan828/beidou-layout/app/service/admin-service/internal/service/hello"
	storagesvcapp "github.com/xiongpan828/beidou-layout/app/service/admin-service/internal/service/storage"
	"github.com/xiongpan828/beidou-layout/app/service/admin-service/pkg/constants"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
	"github.com/xiongpan828/beidou-layout/pkg/loggingx"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// newServer 装配 admin-service 依赖链：config → initializer.Clients → service → http server。
// 两个模块都是纯代理，没有 biz；需要手写方法时照 api-service 加 model / biz。
func newServer() (*khttp.Server, func(), error) {
	var bc conf.Config
	if err := configx.Load(flagconf, &bc); err != nil {
		return nil, nil, err
	}
	loggingx.Init(constants.ServiceName)

	// 下游连接 + 代理路径
	cleanup, err := initializer.NewClients(context.Background(), &bc)
	if err != nil {
		return nil, nil, err
	}

	srv := server.NewHTTPServer(&bc, hellosvcapp.NewHello(), storagesvcapp.NewStorage())
	return srv, cleanup, nil
}
