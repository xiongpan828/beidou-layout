package configx_test

import (
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	adminconf "github.com/xiongpan828/beidou-layout/api/gen/go/config/admin_service"
	apiconf "github.com/xiongpan828/beidou-layout/api/gen/go/config/api_service"
	helloconf "github.com/xiongpan828/beidou-layout/api/gen/go/config/hello_service"
	storageconf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
)

// 仓库里的本地服务配置都能按 conf.proto 加载并通过校验：
// 写错字段、缺必填项等会在这里失败，而不是上线后被静默忽略
func TestRepoConfigsLoad(t *testing.T) {
	root := filepath.Join("..", "..")
	services := map[string]func() proto.Message{
		"admin-service":   func() proto.Message { return &adminconf.Config{} },
		"api-service":     func() proto.Message { return &apiconf.Config{} },
		"hello-service":   func() proto.Message { return &helloconf.Config{} },
		"storage-service": func() proto.Message { return &storageconf.Config{} },
	}
	for svc, newCfg := range services {
		path := filepath.Join(root, "app", "service", svc, "configs", "config.yaml")
		if err := configx.Load(path, newCfg()); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}
