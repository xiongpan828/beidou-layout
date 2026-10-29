package storage

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
)

// CreateUploadURL 登记 PENDING 文件并签发上传地址
func (s *StorageService) CreateUploadURL(ctx context.Context, req *storagesvc.CreateUploadURLReq) (*storagesvc.CreateUploadURLReply, error) {
	u, err := s.biz.CreateUploadURL(ctx, model.CreateUploadParam{
		Scope:    req.GetScope(),
		Filename: req.GetFilename(),
	})
	if err != nil {
		return nil, err
	}
	return &storagesvc.CreateUploadURLReply{Key: u.Key, UploadUrl: u.URL, ExpireAt: timestamppb.New(u.ExpireAt)}, nil
}

// GetFile 文件信息 + 访问地址
func (s *StorageService) GetFile(ctx context.Context, req *storagesvc.GetFileReq) (*storagesvc.GetFileReply, error) {
	f, err := s.biz.GetFile(ctx, req.GetKey())
	if err != nil {
		return nil, err
	}
	return &storagesvc.GetFileReply{File: s.toProtoFile(f)}, nil
}

// ListProvider 供应商列表
func (s *StorageService) ListProvider(ctx context.Context, _ *storagesvc.ListProviderReq) (*storagesvc.ListProviderReply, error) {
	list, err := s.biz.ListProvider(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*storagesvc.Provider, 0, len(list))
	for _, p := range list {
		out = append(out, toProtoProvider(p))
	}
	return &storagesvc.ListProviderReply{Providers: out}, nil
}

// CreateProvider 创建供应商
func (s *StorageService) CreateProvider(ctx context.Context, req *storagesvc.CreateProviderReq) (*storagesvc.CreateProviderReply, error) {
	p, err := s.biz.CreateProvider(ctx, model.CreateProviderParam{Code: req.GetCode(), Name: req.GetName(), Type: req.GetType(), Config: req.GetConfig()})
	if err != nil {
		return nil, err
	}
	return &storagesvc.CreateProviderReply{Provider: toProtoProvider(p)}, nil
}

// UpdateProvider 更新供应商
func (s *StorageService) UpdateProvider(ctx context.Context, req *storagesvc.UpdateProviderReq) (*storagesvc.UpdateProviderReply, error) {
	p, err := s.biz.UpdateProvider(ctx, model.UpdateProviderParam{Code: req.GetCode(), Name: req.GetName(), Config: req.GetConfig()})
	if err != nil {
		return nil, err
	}
	return &storagesvc.UpdateProviderReply{Provider: toProtoProvider(p)}, nil
}

// DeleteProvider 删除供应商
func (s *StorageService) DeleteProvider(ctx context.Context, req *storagesvc.DeleteProviderReq) (*storagesvc.DeleteProviderReply, error) {
	if err := s.biz.DeleteProvider(ctx, req.GetCode()); err != nil {
		return nil, err
	}
	return &storagesvc.DeleteProviderReply{}, nil
}
