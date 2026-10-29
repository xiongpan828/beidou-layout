// Package storage 实现 B 端存储 HTTP 接口
// 全部方法未实现，由 pkg/proxyx 中间件按配置 proxy.downstreams 自动转发到下游 storage-service
package storage

import (
	adminstorage "github.com/xiongpan828/beidou-layout/api/gen/go/service/admin_service/admin_storage"
)

// Storage 实现 service.admin_service.admin_storage.Storage 接口（纯代理，无手写方法）
type Storage struct {
	adminstorage.UnimplementedStorageServer
}

// NewStorage 创建 B 端存储服务
func NewStorage() *Storage {
	return &Storage{}
}
