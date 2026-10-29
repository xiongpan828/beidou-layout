// Package configx 配置加载与 protovalidate 校验（kratos config + buf validate）
package configx

import (
	"os"
	"path/filepath"
	"strings"

	"buf.build/go/protovalidate"
	kconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"google.golang.org/protobuf/proto"
)

// localPath 本机覆盖配置的路径：configs/config.yaml → configs/config.local.yaml。
// 这个文件不进 git（见 .gitignore），用来放密钥等只属于本机 / 本环境的值。
func localPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".local" + ext
}

// Load 加载配置文件到 cfg（*conf.Config 指针），并用 protovalidate 校验。
// 同目录下有 config.local.yaml 时叠加在上面（同名字段以它为准），例如只写某个密钥。
// 校验规则定义在各服务 conf.proto 的 buf validate 约束中。
func Load(path string, cfg proto.Message) error {
	sources := []kconfig.Source{file.NewSource(path)}
	if local := localPath(path); fileExists(local) {
		sources = append(sources, file.NewSource(local))
	}
	c := kconfig.New(kconfig.WithSource(sources...))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		return err
	}
	if err := c.Scan(cfg); err != nil {
		return err
	}
	return protovalidate.Validate(cfg)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
