// Package hello 主服务命令
package hello

import (
	"github.com/go-kratos/kratos/v3"
	"github.com/spf13/cobra"

	"github.com/xiongpan828/beidou-layout/app/service/hello-service/internal/server"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/pkg/constants"
)

var flagconf string

func init() {
	Cmd.Flags().StringVarP(&flagconf, "conf", "c", "configs/config.yaml", "config path")
}

// Cmd 主服务命令
var Cmd = &cobra.Command{
	Use:   "hello",
	Short: "start hello-service",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, cleanup, err := newService()
		if err != nil {
			return err
		}
		defer cleanup()
		app := kratos.New(kratos.Name(constants.ServiceName), kratos.Server(server.NewGRPCServer(cfg, svc)))
		return app.Run()
	},
}
