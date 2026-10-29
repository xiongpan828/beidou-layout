package storage

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	common "github.com/xiongpan828/beidou-layout/api/gen/go/service/common"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/enumx"
	"github.com/xiongpan828/beidou-layout/pkg/storagex"
)

// toProtoFile 文件 + 访问地址
func (s *StorageService) toProtoFile(f *model.File) *storagesvc.File {
	return &storagesvc.File{
		Id:          f.ID,
		Key:         f.Key,
		Filename:    f.Filename,
		Size:        f.Size,
		ContentType: f.ContentType,
		Status:      common.FileStatus(enumx.From(common.FileStatus_value, f.Status)),
		Scope:       f.Scope,
		CreatedAt:   timestamppb.New(f.CreatedAt),
		UpdatedAt:   timestamppb.New(f.UpdatedAt),
		Url:         s.biz.FileURL(f),
	}
}

// toProtoProvider 供应商（配置去掉密钥再回显）
func toProtoProvider(p *model.Provider) *storagesvc.Provider {
	return &storagesvc.Provider{
		Id:        p.ID,
		Name:      p.Name,
		Code:      p.Code,
		Type:      p.Type,
		Config:    storagex.Redact(p.Type, p.Config),
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
}
