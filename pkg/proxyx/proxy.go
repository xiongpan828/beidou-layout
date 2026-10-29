// Package proxyx 提供 HTTP/gRPC 转 gRPC 的代理中间件：
// service 方法未实现（返回 codes.Unimplemented）时，自动反射调用下游 client 的同名方法转发。
// 代理路径通过 AddClient 显式注册（支持 filepath.Match 通配符），由各服务配置驱动。
package proxyx

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// routes 路由表：rpc 路径模式 → 下游 client
var routes = map[string]any{}

// AddClient 注册下游 client 与代理路径（paths 支持 filepath.Match 模式）
// 例：AddClient(client, []string{"/service.admin_service.admin_hello.Hello/*"})
func AddClient(client any, paths []string) {
	for _, p := range paths {
		routes[p] = client
	}
}

// Server 返回代理中间件
func Server() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := handler(ctx, req)
			if !isUnimplemented(err) {
				return reply, err
			}
			// 方法未实现 → 走代理转发
			return forward(ctx, req)
		}
	}
}

func isUnimplemented(err error) bool {
	return err != nil && status.Code(err) == codes.Unimplemented
}

// forward 反射调用下游 client 的同名方法
func forward(ctx context.Context, req any) (any, error) {
	op := operation(ctx)
	client, ok := matchClient(op)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "no proxy route for %s", op)
	}
	method := methodName(op)
	m := reflect.ValueOf(client).MethodByName(method)
	if !m.IsValid() {
		return nil, status.Errorf(codes.Unimplemented, "proxy method %s not found", method)
	}
	res := m.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(req)})
	if len(res) != 2 {
		return nil, status.Error(codes.Internal, "proxy call result invalid")
	}
	if err, ok := res[1].Interface().(error); ok && err != nil {
		return nil, err
	}
	return res[0].Interface(), nil
}

func operation(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return tr.Operation()
}

func matchClient(op string) (any, bool) {
	for pattern, client := range routes {
		if ok, _ := filepath.Match(pattern, op); ok {
			return client, true
		}
	}
	return nil, false
}

// methodName 取 operation 的方法名（/pkg.Service/Method → Method）
func methodName(op string) string {
	return op[strings.LastIndex(op, "/")+1:]
}
