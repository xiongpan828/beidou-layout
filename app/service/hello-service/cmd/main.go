// Package main hello-service 入口（cobra 多命令）
package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/xiongpan828/beidou-layout/app/service/hello-service/cmd/gen"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/cmd/hello"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/cmd/migrate"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/pkg/constants"
)

var rootCmd = &cobra.Command{Use: constants.ServiceName}

func init() {
	rootCmd.AddCommand(hello.Cmd)
	rootCmd.AddCommand(migrate.Cmd)
	rootCmd.AddCommand(gen.Cmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
