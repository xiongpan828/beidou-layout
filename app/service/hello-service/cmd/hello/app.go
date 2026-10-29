package hello

import (
	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/internal/biz"
	hellosvc "github.com/xiongpan828/beidou-layout/app/service/hello-service/internal/service/hello"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/pkg/constants"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
	"github.com/xiongpan828/beidou-layout/pkg/dbx"
	"github.com/xiongpan828/beidou-layout/pkg/loggingx"
)

// newService 装配 hello-service 依赖链：config → db → biz → service
func newService() (*hellosvc.HelloService, *conf.Config, func(), error) {
	var cfg conf.Config
	if err := configx.Load(flagconf, &cfg); err != nil {
		return nil, nil, nil, err
	}
	loggingx.Init(constants.ServiceName)

	db, closeDB, err := dbx.NewGorm(cfg.GetDsn())
	if err != nil {
		return nil, nil, nil, err
	}
	return hellosvc.NewHelloService(biz.NewBiz(db)), &cfg, closeDB, nil
}
