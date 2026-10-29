// Package initializer api-service 初始化：下游 client 构造 + 代理路径注册
package initializer

import (
	"context"

	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/api_service"
	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/pkg/proxyx"
)

// Clients api-service 下游 client 集合（biz 用）
type Clients struct {
	Hello hellosvc.HelloServiceClient
}

// NewClients 连接下游并注册代理路径（未实现的方法由 pkg/proxyx 按配置 proxy.downstreams 转发）。
// 新增下游：Clients 加字段，照 hello 再连一个，配置加地址。
func NewClients(ctx context.Context, c *conf.Config) (*Clients, func(), error) {
	helloConn, err := kgrpc.NewClient(ctx, kgrpc.WithEndpoint(c.GetHelloGrpcAddr()))
	if err != nil {
		return nil, nil, err
	}
	clients := &Clients{Hello: hellosvc.NewHelloServiceClient(helloConn)}
	proxyx.AddClient(clients.Hello, c.GetProxy().GetDownstreams()["hello"].GetPaths())
	return clients, func() { _ = helloConn.Close() }, nil
}
