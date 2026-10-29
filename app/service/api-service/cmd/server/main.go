// Package main api-service 入口
package main

import (
	"flag"
	"log"

	"github.com/go-kratos/kratos/v3"

	"github.com/xiongpan828/beidou-layout/app/service/api-service/pkg/constants"
)

var flagconf string

func init() {
	flag.StringVar(&flagconf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
}

func main() {
	flag.Parse()

	// newServer 装配依赖 → kratos 启动
	srv, cleanup, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	app := kratos.New(
		kratos.Name(constants.ServiceName),
		kratos.Server(srv),
	)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
