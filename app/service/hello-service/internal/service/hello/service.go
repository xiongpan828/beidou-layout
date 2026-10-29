// Package hello 实现 hello-service 的 gRPC 接口
package hello

import (
	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/model"
)

// HelloService 实现 service.hello_service.HelloService
type HelloService struct {
	hellosvc.UnimplementedHelloServiceServer
	biz model.IHelloBiz
}

// NewHelloService 创建问候服务
func NewHelloService(biz model.IHelloBiz) *HelloService { return &HelloService{biz: biz} }
