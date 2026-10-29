package storage

import (
	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/internal/biz"
	storagesvc "github.com/xiongpan828/beidou-layout/app/service/storage-service/internal/service/storage"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/pkg/constants"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
	"github.com/xiongpan828/beidou-layout/pkg/dbx"
	"github.com/xiongpan828/beidou-layout/pkg/loggingx"
)

// newService 装配 storage-service 依赖链：config → db → biz → service
func newService() (*storagesvc.StorageService, *conf.Config, func(), error) {
	var cfg conf.Config
	if err := configx.Load(flagconf, &cfg); err != nil {
		return nil, nil, nil, err
	}
	loggingx.Init(constants.ServiceName)

	db, closeDB, err := dbx.NewGorm(cfg.GetDsn())
	if err != nil {
		return nil, nil, nil, err
	}
	return storagesvc.NewStorageService(biz.NewBiz(db, &cfg)), &cfg, closeDB, nil
}
