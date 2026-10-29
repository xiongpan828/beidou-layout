#!/usr/bin/env bash
# 把 proto 生成插件安装到 tools/（git 忽略），版本固定在这里：生成物 header 里的版本以此为准，升级后 make api 重新生成。
set -euo pipefail

cd "$(dirname "$0")/../.."
export GOBIN="$PWD/tools"

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
# kratos v3 的 errors / http 插件只有伪版本
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v3@v3.0.0-20260626125723-668db92c2c00
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@v3.0.0-20260626125723-668db92c2c00
# TS 请求桩：kratos fork 的 protoc-gen-typescript-http
go install github.com/go-kratos/protoc-gen-typescript-http@v0.0.0-20260526061946-7cb5d90103c2
