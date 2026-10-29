// Package initializer admin-service 初始化：下游连接 + 代理路径注册
package initializer

import (
	"context"

	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"google.golang.org/grpc"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/admin_service"
	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/pkg/proxyx"
)

// NewClients 连接下游并注册代理路径（admin-service 的方法全部由 pkg/proxyx 按配置 proxy.downstreams 转发，
// 不需要持有 client；有手写方法时照 api-service 返回 Clients 给 biz）。任一连接失败时关闭已建立的连接。
func NewClients(ctx context.Context, c *conf.Config) (func(), error) {
	var conns []*grpc.ClientConn
	cleanup := func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}
	dial := func(addr string) (*grpc.ClientConn, error) {
		conn, err := kgrpc.NewClient(ctx, kgrpc.WithEndpoint(addr))
		if err == nil {
			conns = append(conns, conn)
		}
		return conn, err
	}
	downstreams := c.GetProxy().GetDownstreams()

	helloConn, err := dial(c.GetHelloGrpcAddr())
	if err != nil {
		return nil, err
	}
	proxyx.AddClient(hellosvc.NewHelloServiceClient(helloConn), downstreams["hello"].GetPaths())

	storageConn, err := dial(c.GetStorageGrpcAddr())
	if err != nil {
		cleanup()
		return nil, err
	}
	proxyx.AddClient(storagesvc.NewStorageServiceClient(storageConn), downstreams["storage"].GetPaths())
	return cleanup, nil
}
