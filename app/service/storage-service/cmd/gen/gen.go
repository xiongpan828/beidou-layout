// Package gen gorm gen 生成 dao 命令
package gen

import (
	"github.com/spf13/cobra"
	"gorm.io/gen"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
	"github.com/xiongpan828/beidou-layout/pkg/dbx"
)

var (
	output   string
	flagconf string
)

func init() {
	Cmd.Flags().StringVarP(&output, "output", "o", "app/service/storage-service/internal/dao", "output directory")
	Cmd.Flags().StringVarP(&flagconf, "conf", "c", "configs/config.yaml", "config path")
}

// Cmd gorm gen 命令
var Cmd = &cobra.Command{
	Use:   "gen",
	Short: "generate gorm dao",
	RunE: func(cmd *cobra.Command, args []string) error {
		var cfg conf.Config
		if err := configx.Load(flagconf, &cfg); err != nil {
			return err
		}
		// gen 只解析模型、不查库：不需要起数据库
		db, cleanup, err := dbx.NewGormOffline(cfg.GetDsn())
		if err != nil {
			return err
		}
		defer cleanup()
		g := gen.NewGenerator(gen.Config{OutPath: output, Mode: gen.WithQueryInterface})
		g.UseDB(db)
		for m, fn := range model.GetModels() {
			g.ApplyInterface(fn, m)
		}
		g.Execute()
		return nil
	},
}
