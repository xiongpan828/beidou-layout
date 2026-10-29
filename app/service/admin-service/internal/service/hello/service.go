// Package hello 实现 B 端问候 HTTP 接口
// 全部方法未实现，由 pkg/proxyx 中间件按配置 proxy.downstreams 自动转发到下游 hello-service
package hello

import (
	adminhello "github.com/xiongpan828/beidou-layout/api/gen/go/service/admin_service/admin_hello"
)

// Hello 实现 service.admin_service.admin_hello.Hello 接口（纯代理，无手写方法）
type Hello struct {
	adminhello.UnimplementedHelloServer
}

// NewHello 创建 B 端问候服务
func NewHello() *Hello {
	return &Hello{}
}
