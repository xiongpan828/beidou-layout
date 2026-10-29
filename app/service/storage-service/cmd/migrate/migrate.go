// Package migrate 迁移命令
package migrate

import (
	"github.com/spf13/cobra"

	conf "github.com/xiongpan828/beidou-layout/api/gen/go/config/storage_service"
	svcmigrate "github.com/xiongpan828/beidou-layout/app/service/storage-service/migrate"
	"github.com/xiongpan828/beidou-layout/pkg/configx"
	"github.com/xiongpan828/beidou-layout/pkg/dbx"
)

var flagconf string

func init() {
	Cmd.Flags().StringVarP(&flagconf, "conf", "c", "configs/config.yaml", "config path")
}

// Cmd 迁移命令
var Cmd = &cobra.Command{
	Use:   "migrate",
	Short: "run database migration",
	RunE: func(cmd *cobra.Command, args []string) error {
		var cfg conf.Config
		if err := configx.Load(flagconf, &cfg); err != nil {
			return err
		}
		db, cleanup, err := dbx.NewGorm(cfg.GetDsn())
		if err != nil {
			return err
		}
		defer cleanup()
		return svcmigrate.Migrate(db)
	},
}
