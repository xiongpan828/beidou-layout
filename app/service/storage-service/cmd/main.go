// Package main storage-service 入口（cobra 多命令）
package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/xiongpan828/beidou-layout/app/service/storage-service/cmd/gen"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/cmd/migrate"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/cmd/storage"
	"github.com/xiongpan828/beidou-layout/app/service/storage-service/pkg/constants"
)

var rootCmd = &cobra.Command{Use: constants.ServiceName}

func init() {
	rootCmd.AddCommand(storage.Cmd)
	rootCmd.AddCommand(migrate.Cmd)
	rootCmd.AddCommand(gen.Cmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
