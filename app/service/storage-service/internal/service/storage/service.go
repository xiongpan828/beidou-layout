// Package storage 实现 storage-service 的 gRPC 接口与文件 HTTP 读写
package storage

import (
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
)

// StorageService 实现 service.storage_service.StorageService（gRPC）+ 文件读写（HTTP，见 http.go）
type StorageService struct {
	storagesvc.UnimplementedStorageServiceServer
	biz model.IStorageBiz
}

// NewStorageService 创建存储服务
func NewStorageService(biz model.IStorageBiz) *StorageService { return &StorageService{biz: biz} }
